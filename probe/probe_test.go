package probe_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/macrat/ayd/probe"
	"github.com/macrat/ayd/store"
)

func TestTargetURLNormalize(t *testing.T) {
	tests := []struct {
		Input string
		Want  url.URL
	}{
		{"ping:example.com", url.URL{Scheme: "ping", Opaque: "example.com"}},
		{"ping://example.com:123/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "ping", Opaque: "example.com"}},

		{"http://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "http", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},
		{"https://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "https", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},

		{"http-get://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "http-get", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},
		{"https-post://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "https-post", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},
		{"http-head://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "http-head", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},
		{"https-options://example.com/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "https-options", Host: "example.com", Path: "/foo/bar", RawQuery: "hoge=fuga", Fragment: "piyo"}},

		{"tcp:example.com:80", url.URL{Scheme: "tcp", Opaque: "example.com:80"}},
		{"tcp://example.com:80/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "tcp", Opaque: "example.com:80"}},

		{"dns:example.com", url.URL{Scheme: "dns", Opaque: "example.com"}},
		{"dns://example.com:80/foo/bar?hoge=fuga#piyo", url.URL{Scheme: "dns", Opaque: "example.com"}},

		{"exec:foo.sh", url.URL{Scheme: "exec", Opaque: "foo.sh"}},
		{"exec:./foo.sh", url.URL{Scheme: "exec", Opaque: "./foo.sh"}},
		{"exec:/foo/bar.sh", url.URL{Scheme: "exec", Opaque: "/foo/bar.sh"}},
		{"exec:///foo/bar.sh", url.URL{Scheme: "exec", Opaque: "/foo/bar.sh"}},
		{"exec:foo.sh?hoge=fuga#piyo", url.URL{Scheme: "exec", Opaque: "foo.sh", RawQuery: "hoge=fuga", Fragment: "piyo"}},
		{"exec:/foo/bar.sh?hoge=fuga#piyo", url.URL{Scheme: "exec", Opaque: "/foo/bar.sh", RawQuery: "hoge=fuga", Fragment: "piyo"}},

		{"source:./stub/healthy-list.txt", url.URL{Scheme: "source", Opaque: "./stub/healthy-list.txt"}},
	}

	for _, tt := range tests {
		p, err := probe.Get(tt.Input)
		if err != nil {
			t.Errorf("%#v: failed to parse: %#s", tt.Input, err)
			continue
		}

		u := p.Target()

		if u.Scheme != tt.Want.Scheme {
			t.Errorf("%#v expected scheme %#v but go %#v", tt.Input, tt.Want.Scheme, u.Scheme)
		}

		if u.Opaque != tt.Want.Opaque {
			t.Errorf("%#v expected opaque %#v but go %#v", tt.Input, tt.Want.Opaque, u.Opaque)
		}

		if u.Host != tt.Want.Host {
			t.Errorf("%#v expected host %#v but go %#v", tt.Input, tt.Want.Host, u.Host)
		}

		if u.Path != tt.Want.Path {
			t.Errorf("%#v expected path %#v but go %#v", tt.Input, tt.Want.Path, u.Path)
		}

		if u.Fragment != tt.Want.Fragment {
			t.Errorf("%#v expected fragment %#v but go %#v", tt.Input, tt.Want.Fragment, u.Fragment)
		}

		if u.RawQuery != tt.Want.RawQuery {
			t.Errorf("%#v expected query %#v but go %#v", tt.Input, tt.Want.RawQuery, u.RawQuery)
		}
	}
}

type ProbeTest struct {
	Target         string
	Status         store.Status
	MessagePattern string
}

func AssertProbe(t *testing.T, tests []ProbeTest) {
	for _, tt := range tests {
		t.Run(tt.Target, func(t *testing.T) {
			p, err := probe.Get(tt.Target)
			if err != nil {
				t.Fatalf("failed to create probe: %s", err)
			}

			if p.Target().String() != tt.Target {
				t.Fatalf("got unexpected probe: %s", p.Target())
			}

			rs := p.Check()

			if len(rs) != 1 {
				t.Fatalf("got unexpected number of results: %d", len(rs))
			}

			r := rs[0]
			if r.Target.String() != tt.Target {
				t.Errorf("got a record of unexpected target: %s", r.Target)
			}
			if r.Status != tt.Status {
				t.Errorf("expected status is %s but got %s", tt.Status, r.Status)
			}
			if ok, _ := regexp.MatchString("^"+tt.MessagePattern+"$", r.Message); !ok {
				t.Errorf("expected message is match to %#v but got %#v", tt.MessagePattern, r.Message)
			}
		})
	}
}

func RunDummyHTTPServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	})
	mux.HandleFunc("/redirect/ok", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusFound)
	})
	mux.HandleFunc("/redirect/error", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/error", http.StatusFound)
	})
	mux.HandleFunc("/redirect/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirect/loop", http.StatusFound)
	})
	mux.HandleFunc("/only/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/only/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/only/head", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/only/options", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "OPTIONS" {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	return httptest.NewServer(mux)
}
