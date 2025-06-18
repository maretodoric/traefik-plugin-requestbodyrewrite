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
    // Regex-based rewrite rules.
    Rewrites []Rewrite `json:"rewrites,omitempty"`
    // HTTP methods to apply rewrites on, empty means all.
    Methods  []string  `json:"methods,omitempty"`
}

// Rewrite defines a regex and its replacement text.
type Rewrite struct {
    Regex       string `json:"regex,omitempty"`
    Replacement string `json:"replacement,omitempty"`
}

// CreateConfig returns default Config.
func CreateConfig() *Config {
    return &Config{}
}

// compiled holds a compiled regex and replacement.
type compiled struct {
    re  *regexp.Regexp
    rep string
}

// RequestBodyRewrite plugin.
type RequestBodyRewrite struct {
    next     http.Handler
    name     string
    rules    []compiled
    methods  map[string]struct{}
}

// New instantiates the plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    // Compile regex rules
    var rules []compiled
    for _, r := range config.Rewrites {
        re, err := regexp.Compile(r.Regex)
        if err != nil {
            return nil, err
        }
        rules = append(rules, compiled{re: re, rep: r.Replacement})
    }
    // Build method filter
    methodsSet := make(map[string]struct{})
    for _, m := range config.Methods {
        methodsSet[strings.ToUpper(m)] = struct{}{}
    }
    return &RequestBodyRewrite{
        next:    next,
        name:    name,
        rules:   rules,
        methods: methodsSet,
    }, nil
}

// ServeHTTP applies regex rewrites to request body.
func (p *RequestBodyRewrite) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // Skip methods not in the set
    if len(p.methods) > 0 {
        if _, ok := p.methods[req.Method]; !ok {
            p.next.ServeHTTP(w, req)
            return
        }
    }
    if req.Body == nil {
        p.next.ServeHTTP(w, req)
        return
    }
    // Read full body
    origBody, err := ioutil.ReadAll(req.Body)
    if err != nil {
        // if error, reset and proceed
        req.Body = io.NopCloser(bytes.NewReader(origBody))
        p.next.ServeHTTP(w, req)
        return
    }
    req.Body.Close()
    // Apply rewrites
    bodyStr := string(origBody)
    for _, rule := range p.rules {
        bodyStr = rule.re.ReplaceAllString(bodyStr, rule.rep)
    }
    newBytes := []byte(bodyStr)
    // Set new body and headers
    req.Body = io.NopCloser(bytes.NewReader(newBytes))
    req.ContentLength = int64(len(newBytes))
    req.Header.Set("Content-Length", strconv.Itoa(len(newBytes)))
    // Call next handler
    p.next.ServeHTTP(w, req)
}