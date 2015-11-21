package mux

import (
	"golang.org/x/net/context"
	"log"
	"net/http"
	"regexp"
	"regexp/syntax"
	"sync"
)

type Placeholder map[string]string

func matchRequest(re *regexp.Regexp, req *http.Request) (bool, Placeholder) {
	switch matches := re.FindStringSubmatch(req.URL.Path); len(matches) {
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

func (mux *ServeMux) Handle(pattern string, handler http.Handler) {
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
	mux.mu.RLock()
	for re, handler := range mux.handlers {
		if ok, placeholder := matchRequest(re, req); ok {
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
