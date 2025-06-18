package traefik_plugin_requestbodyrewrite

import (
    "bytes"
    "context"
    "io"
    "io/ioutil"
    "net/http"
    "regexp"
    "strconv"
    "strings"
)

// Config holds plugin configuration.
type Config struct {
    // A list of rewrite rules.
    Rewrites []Rewrite `json:"rewrites,omitempty"`
}

// Rewrite defines a single rewrite rule with optional filters.
type Rewrite struct {
    // Regex to match in the body.
    Regex       string   `json:"regex,omitempty"`
    // Replacement for matches.
    Replacement string   `json:"replacement,omitempty"`
    // Optional HTTP methods to apply this rule (e.g. ["POST","PUT"]).
    Methods      []string `json:"methods,omitempty"`
    // Optional Content-Types (media), e.g. ["application/json"].
    ContentTypes []string `json:"contentTypes,omitempty"`
    // Optional path regex; only apply if request URL path matches.
    PathRegex    string   `json:"pathRegex,omitempty"`
}

// CreateConfig returns a default Config.
func CreateConfig() *Config {
    return &Config{}
}

// compiledRule holds a compiled rewrite rule and its filters.
type compiledRule struct {
    re           *regexp.Regexp
    rep          string
    methods      map[string]struct{}
    contentTypes map[string]struct{}
    pathRe       *regexp.Regexp
}

// RequestBodyRewrite is the middleware instance.
type RequestBodyRewrite struct {
    next  http.Handler
    name  string
    rules []compiledRule
}

// New constructs a RequestBodyRewrite middleware from config.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    var rules []compiledRule
    for _, r := range config.Rewrites {
        // Compile main regex
        mainRe, err := regexp.Compile(r.Regex)
        if err != nil {
            return nil, err
        }
        // Build methods set
        methodsSet := make(map[string]struct{})
        for _, m := range r.Methods {
            methodsSet[strings.ToUpper(m)] = struct{}{}
        }
        // Build content types set
        ctSet := make(map[string]struct{})
        for _, ct := range r.ContentTypes {
            media := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
            ctSet[media] = struct{}{}
        }
        // Compile path regex if provided
        var pathRe *regexp.Regexp
        if r.PathRegex != "" {
            pr, err := regexp.Compile(r.PathRegex)
            if err != nil {
                return nil, err
            }
            pathRe = pr
        }
        rules = append(rules, compiledRule{
            re: mainRe, rep: r.Replacement,
            methods: methodsSet, contentTypes: ctSet, pathRe: pathRe,
        })
    }
    return &RequestBodyRewrite{next: next, name: name, rules: rules}, nil
}

// ServeHTTP reads, conditionally rewrites, and forwards the request body.
func (p *RequestBodyRewrite) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    if req.Body == nil {
        p.next.ServeHTTP(w, req)
        return
    }
    // Read full body
    origBody, err := ioutil.ReadAll(req.Body)
    if err != nil {
        req.Body = io.NopCloser(bytes.NewReader(origBody))
        p.next.ServeHTTP(w, req)
        return
    }
    req.Body.Close()
    bodyStr := string(origBody)

    // Apply each rewrite rule in order
    for _, rule := range p.rules {
        // Method filter
        if len(rule.methods) > 0 {
            if _, ok := rule.methods[req.Method]; !ok {
                continue
            }
        }
        // Content-Type filter
        if len(rule.contentTypes) > 0 {
            ct := req.Header.Get("Content-Type")
            media := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
            if _, ok := rule.contentTypes[media]; !ok {
                continue
            }
        }
        // Path filter
        if rule.pathRe != nil {
            if !rule.pathRe.MatchString(req.URL.Path) {
                continue
            }
        }
        // Perform replacement
        bodyStr = rule.re.ReplaceAllString(bodyStr, rule.rep)
    }
    newBytes := []byte(bodyStr)
    // Replace body and adjust headers
    req.Body = io.NopCloser(bytes.NewReader(newBytes))
    req.ContentLength = int64(len(newBytes))
    req.Header.Set("Content-Length", strconv.Itoa(len(newBytes)))

    // Continue processing
    p.next.ServeHTTP(w, req)
}