package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Versions compare by their numbers, part by part: 0.1.10 is later than 0.1.9.
func TestNewerVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.1.10", "0.1.9", true},
		{"0.1.5", "0.1.5", false},
		{"0.1.4", "0.1.5", false},
		{"v0.2", "0.1.9", true},
		{"1.0", "1.0.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// The newest release is read from where /releases/latest redirects: its tag
// names the version, and the APK is named after it.
func TestLatestAPKReadsTheTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/"+updateRepo+"/releases/tag/v0.1.9", http.StatusFound)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rel, err := latestAPK(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.1.9" || rel.URL != updateDownload+"v0.1.9/scriptboard-0.1.9.apk" {
		t.Errorf("release %+v", rel)
	}
}

// A release that does not redirect to a tag is an error, not a version.
func TestLatestAPKWithoutATagIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := latestAPK(ctx, srv.Client(), srv.URL); err == nil {
		t.Error("no error for a page without a release")
	}
}

// Only a release later than the running version is offered.
func TestOnlyANewerAPKIsOffered(t *testing.T) {
	a := newTestApp(t)
	a.apkRel = &apkRelease{Version: versionString(), URL: "x"}
	if a.apkNewer() != nil {
		t.Error("the running version is offered as an update")
	}
	a.apkRel = &apkRelease{Version: "9.9.9", URL: "x"}
	if a.apkNewer() == nil {
		t.Error("a newer release is not offered")
	}
}
