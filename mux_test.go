package mux

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func serve(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	})
}

var serveMuxRegisters = []struct {
	pattern string
	handler http.Handler
}{
	{`^/foo$`, serve(200)},
	{`^/foo/(?P<bar>[[:alnum:]]+)$`, serve(200)},
}

var serveMuxTests = []struct {
	method string
	path   string

	code        int
	placeholder Placeholder
}{
	{"GET", "/", 404, nil},
	{"GET", "/foo", 200, nil},
	{"GET", "/foo/10", 200, Placeholder{"bar": "10"}},
}

func TestServeMux(t *testing.T) {
	mux := NewServeMux()
	for _, reg := range serveMuxRegisters {
		mux.Handle(reg.pattern, reg.handler)
	}
	for i, test := range serveMuxTests {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest(test.method, "http://www.example.com"+test.path, nil)
		mux.ServeHTTP(rr, req)
		if code := rr.Code; code != test.code {
			t.Errorf("%d: want code to be %d, got %d", i, test.code, code)
		}
		if placeholder := FromRequest(req); !reflect.DeepEqual(placeholder, test.placeholder) {
			t.Errorf("%d: want placeholder to be %v, got %v", i, test.placeholder, placeholder)
		}
	}
}

var serveMuxBenchmark = struct {
	method string
	path   string
}{
	"GET",
	"/foo/10",
}

func BenchmarkServeMux(b *testing.B) {
	mux := NewServeMux()
	for _, reg := range serveMuxRegisters {
		mux.Handle(reg.pattern, reg.handler)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rr := httptest.NewRecorder()
			method := serveMuxBenchmark.method
			url := "http://www.example.com" + serveMuxBenchmark.path
			req, _ := http.NewRequest(method, url, nil)
			mux.ServeHTTP(rr, req)
			var _ = FromRequest(req)
		}
	})
}
