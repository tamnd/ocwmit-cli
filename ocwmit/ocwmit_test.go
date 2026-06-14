package ocwmit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/ocwmit-cli/ocwmit"
)

const testXML = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex>
  <sitemap>
    <loc>https://ocw.mit.edu/courses/10-01-ethics-for-engineers-artificial-intelligence-spring-2020/sitemap.xml</loc>
  </sitemap>
  <sitemap>
    <loc>https://ocw.mit.edu/courses/1-00-introduction-to-computers-and-engineering-problem-solving-spring-2012/sitemap.xml</loc>
  </sitemap>
  <sitemap>
    <loc>https://ocw.mit.edu/courses/1-010-uncertainty-in-engineering-fall-2008/sitemap.xml</loc>
  </sitemap>
</sitemapindex>`

func newTestClient(srv *httptest.Server) *ocwmit.Client {
	cfg := ocwmit.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	return ocwmit.NewClient(cfg)
}

func TestList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(testXML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	courses, err := c.List(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(courses) != 3 {
		t.Fatalf("got %d courses, want 3", len(courses))
	}

	first := courses[0]
	if first.Number != "10.01" {
		t.Errorf("Number = %q, want %q", first.Number, "10.01")
	}
	if !strings.Contains(first.Title, "Ethics") {
		t.Errorf("Title = %q, want it to contain 'Ethics'", first.Title)
	}
	if first.Term != "spring-2020" {
		t.Errorf("Term = %q, want %q", first.Term, "spring-2020")
	}
	wantURL := "https://ocw.mit.edu/courses/10-01-ethics-for-engineers-artificial-intelligence-spring-2020/"
	if first.URL != wantURL {
		t.Errorf("URL = %q, want %q", first.URL, wantURL)
	}
}

func TestListLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testXML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	courses, err := c.List(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(courses) != 2 {
		t.Fatalf("got %d courses, want 2 (limit=2)", len(courses))
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testXML))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	courses, err := c.Search(context.Background(), "uncertainty", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(courses) != 1 {
		t.Fatalf("got %d courses, want 1", len(courses))
	}
	if courses[0].Number != "1.010" {
		t.Errorf("Number = %q, want %q", courses[0].Number, "1.010")
	}
}
