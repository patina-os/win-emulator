// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fakeImage = strings.Repeat("patina ", 100000)

func fakeGitHub(t *testing.T) {
	sum := sha256.Sum256([]byte(fakeImage))
	mux := http.NewServeMux()
	mux.HandleFunc("/patina-os/patina/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/patina-os/patina/releases/tag/v0.1.0-alpha.3", http.StatusFound)
	})
	mux.HandleFunc("/patina-os/patina/releases/download/v0.1.0-alpha.3/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + isoName + "\n"))
	})
	mux.HandleFunc("/patina-os/patina/releases/download/v0.1.0-alpha.3/"+isoName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, isoName, time.Time{}, strings.NewReader(fakeImage))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	old := github
	github = s.URL
	t.Cleanup(func() { github = old })
}

func TestDownload(t *testing.T) {
	fakeGitHub(t)
	ctx := context.Background()
	c := http.DefaultClient
	tag, err := latestRelease(ctx, c, patinaRepo)
	if err != nil || tag != "v0.1.0-alpha.3" {
		t.Fatalf("latest release %q, %v", tag, err)
	}
	want, err := checksum(ctx, c, releaseAsset(patinaRepo, tag, sumsName), isoName)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), isoName)
	// Half a file left by an interrupted run is continued, not fetched again.
	os.WriteFile(dest+".part", []byte(fakeImage[:len(fakeImage)/2]), 0o644)
	var last, total int64
	err = download(ctx, c, releaseAsset(patinaRepo, tag, isoName), dest, want, func(d, t int64) { last, total = d, t })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != fakeImage {
		t.Error("the downloaded file differs")
	}
	if last != int64(len(fakeImage)) || total != int64(len(fakeImage)) {
		t.Errorf("progress ended at %d of %d, want %d", last, total, len(fakeImage))
	}
	if got, _ := fileSHA256(dest); got != want {
		t.Error("fileSHA256 disagrees with the checksum")
	}
}

func TestDownloadRejectsAWrongChecksum(t *testing.T) {
	fakeGitHub(t)
	dest := filepath.Join(t.TempDir(), isoName)
	err := download(context.Background(), http.DefaultClient,
		releaseAsset(patinaRepo, "v0.1.0-alpha.3", isoName), dest, strings.Repeat("0", 64), nil)
	if err == nil {
		t.Fatal("a wrong checksum is an error")
	}
	if _, err := os.Stat(dest); err == nil {
		t.Error("a file with a wrong checksum is not kept")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("nor is its part")
	}
}
