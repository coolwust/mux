package mux

import (
	"golang.org/x/net/context"
	"log"
	"net/http"
	"regexp"
	"regexp/syntax"
	"sync"
	"path"
)

type Placeholder map[string]string

func matchPath(re *regexp.Regexp, path string) (bool, Placeholder) {
	switch matches := re.FindStringSubmatch(path); len(matches) {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		sre, _ := syntax.Parse(re.String(), syntax.Perl)
		placeholder := make(Placeholder)
		for i, name := range sre.CapNames() {
			if i == 0 {
				continue
			}
			placeholder[name] = matches[i]
		}
		return true, placeholder
	}
}

func toRequest(req *http.Request, placeholder Placeholder) {
	if placeholder == nil || len(placeholder) == 0 {
		return
	}
	header := make([]string, 0)
	for name, value := range placeholder {
		header = append(header, name, value)
	}
	req.Header["X-HTTP-Placeholder"] = header
}

type ServeMux struct {
	handlers map[*regexp.Regexp]http.Handler
	mu       sync.RWMutex
}

func cleanPath(p string) string {
	if p == "" {
		p = "/" + p
	}
	np := path.Clean(p)
	if p[len(p)-1] == '/' && p != "/" {
		np += "/"
	}
	return np
}

func (mux *ServeMux) Handle(pattern string, handler http.Handler) {
	if pattern == "" {
		panic("mux: empty pattern")
	}
	if handler == nil {
		panic("mux: nil handler")
	}
	mux.mu.Lock()
	for re, _ := range mux.handlers {
		if re.String() == pattern {
			log.Panicf("mux: pattern %s already exists", pattern)
		}
	}
	mux.handlers[regexp.MustCompile(pattern)] = handler
	mux.mu.Unlock()
}

func (mux *ServeMux) HandleFunc(pattern string, f func(http.ResponseWriter, *http.Request)) {
	mux.Handle(pattern, http.HandlerFunc(f))
}

func (mux *ServeMux) Handler(req *http.Request) (http.Handler, string, Placeholder) {
	if req.Method != "CONNECT" {
		if path := cleanPath(req.URL.Path); path != req.URL.Path {
			_, pattern, placeholder := mux.handler(path)
			u := req.URL
			u.Path = path
			return http.RedirectHandler((*u).String(), http.StatusMovedPermanently), pattern, placeholder
		}
	}
	return mux.handler(req.URL.Path)
}

func (mux *ServeMux) handler(path string) (http.Handler, string, Placeholder) {
	mux.mu.RLock()
	for re, handler := range mux.handlers {
		if ok, placeholder := matchPath(re, path); ok {
			return handler, re.String(), placeholder
		}
	}
	mux.mu.RUnlock()
	return http.NotFoundHandler(), "", nil
}

func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler, _, placeholder := mux.Handler(req)
	toRequest(req, placeholder)
	handler.ServeHTTP(w, req)
}

func NewServeMux() *ServeMux {
	return &ServeMux{handlers: make(map[*regexp.Regexp]http.Handler)}
}

type key int

const placeholderKey key = iota

func NewContext(ctx context.Context, placeholder Placeholder) context.Context {
	return context.WithValue(ctx, placeholderKey, placeholder)
}

func FromContext(ctx context.Context) Placeholder {
	return ctx.Value(placeholderKey).(Placeholder)
}

func FromRequest(req *http.Request) Placeholder {
	header, ok := req.Header["X-HTTP-Placeholder"]
	if !ok {
		return nil
	}
	placeholder := make(Placeholder)
	for i := 0; i < len(header)-1; i = i + 2 {
		placeholder[header[i]] = header[i+1]
	}
	return placeholder
}
