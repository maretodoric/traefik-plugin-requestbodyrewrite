package requestbodyrewrite

import (
    "bytes"
    "context"
    "io/ioutil"
    "net/http"
    "regexp"
    "strconv"
    "strings"

    "github.com/traefik/plugin-sdk/v2/pkg/transport"
)

// Config holds the plugin configuration.
type Config struct {
    // A list of regex patterns and their replacements.
    Rewrites []Rewrite `json:"rewrites,omitempty"`
    // Optional list of HTTP methods (e.g., ["POST","PUT"]) to apply rewrites to.
    Methods  []string  `json:"methods,omitempty"`
}

// Rewrite defines a single regex + replacement rule.
type Rewrite struct {
    Regex       string `json:"regex,omitempty"`
    Replacement string `json:"replacement,omitempty"`
}

// CreateConfig returns a default configuration.
func CreateConfig() *Config {
    return &Config{}
}

// compiledRewrite holds a compiled regex for fast execution.
type compiledRewrite struct {
    regex       *regexp.Regexp
    replacement string
}

// RequestBodyRewrite is the middleware instance.
type RequestBodyRewrite struct {
    next     http.Handler
    name     string
    rewrites []compiledRewrite
    methods  map[string]struct{}
}

// New builds a new middleware instance.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
    // Compile all regexes up front.
    var rewrites []compiledRewrite
    for _, r := range config.Rewrites {
        re, err := regexp.Compile(r.Regex)
        if err != nil {
            return nil, err
        }
        rewrites = append(rewrites, compiledRewrite{regex: re, replacement: r.Replacement})
    }

    // Build method filter set (if provided).
    methodsSet := make(map[string]struct{})
    for _, m := range config.Methods {
        methodsSet[strings.ToUpper(m)] = struct{}{}
    }

    return &RequestBodyRewrite{
        next:     next,
        name:     name,
        rewrites: rewrites,
        methods:  methodsSet,
    }, nil
}

// ServeHTTP reads and rewrites the request body before passing to the upstream.
func (p *RequestBodyRewrite) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    // If methods filter is set and this method is not in it, skip.
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

    // Read the entire body (handles chunked, etc.).
    bodyBytes, err := transport.ReadBody(req)
    if err != nil {
        // If we can't read, just forward original request.
        p.next.ServeHTTP(w, req)
        return
    }

    // Apply each regex replacement in order.
    bodyStr := string(bodyBytes)
    for _, r := range p.rewrites {
        bodyStr = r.regex.ReplaceAllString(bodyStr, r.replacement)
    }

    // Replace the request body and update headers.
    newBody := []byte(bodyStr)
    req.Body = ioutil.NopCloser(bytes.NewReader(newBody))
    req.ContentLength = int64(len(newBody))
    req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))

    // Call the next handler in the chain.
    p.next.ServeHTTP(w, req)
}