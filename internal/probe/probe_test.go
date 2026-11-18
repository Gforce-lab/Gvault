package probe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gvault/gvault/internal/probe"
)

func TestHTTPProber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	r := probe.HTTPProber{}.Check(context.Background(), srv.URL)
	if !r.OK || r.Simulated {
		t.Fatalf("%+v", r)
	}
}

func TestHTTPProberRejectsEmpty(t *testing.T) {
	r := probe.HTTPProber{}.Check(context.Background(), "")
	if r.OK {
		t.Fatalf("%+v", r)
	}
}
