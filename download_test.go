// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	tag, want, err := imageRelease(ctx, c, patinaRepo, isoName)
	if err != nil || tag != "v0.1.0-alpha.3" {
		t.Fatalf("image release %q, %v", tag, err)
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

// releases serves a latest release without an image (published, its images not yet
// attached) and the API's listing of the ones before it.
func releases(t *testing.T, listing string) {
	sum := sha256.Sum256([]byte(fakeImage))
	mux := http.NewServeMux()
	mux.HandleFunc("/patina-os/patina/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/patina-os/patina/releases/tag/v0.1.0-alpha.5", http.StatusFound)
	})
	mux.HandleFunc("/repos/patina-os/patina/releases", func(w http.ResponseWriter, r *http.Request) {
		if listing == "" {
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		w.Write([]byte(listing))
	})
	for _, tag := range []string{"v0.1.0-alpha.3", "v0.1.0-alpha.4"} {
		mux.HandleFunc("/patina-os/patina/releases/download/"+tag+"/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + isoName + "\n"))
		})
	}
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	oldWeb, oldAPI := github, githubAPI
	github, githubAPI = s.URL, s.URL
	t.Cleanup(func() { github, githubAPI = oldWeb, oldAPI })
}

func TestReleaseFilesAndPages(t *testing.T) {
	pages := `{"theme": "aurora", "pages": [{"id": "theme", "key": "PATINA_THEME"}]}`
	sum := sha256.Sum256([]byte(pages))
	mux := http.NewServeMux()
	mux.HandleFunc("/patina-os/patina/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/patina-os/patina/releases/tag/v0.1.0-alpha.5", http.StatusFound)
	})
	mux.HandleFunc("/patina-os/patina/releases/download/v0.1.0-alpha.5/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n%s  %s\n", strings.Repeat("a", 64), diskImageName, hex.EncodeToString(sum[:]), setupPagesName)
	})
	mux.HandleFunc("/patina-os/patina/releases/download/v0.1.0-alpha.5/"+setupPagesName, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(pages))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	oldGitHub, oldAPI := github, githubAPI
	github, githubAPI = s.URL, s.URL // the API lists no releases here
	t.Cleanup(func() { github, githubAPI = oldGitHub, oldAPI })
	ctx := context.Background()
	tag, sums, err := releaseFiles(ctx, http.DefaultClient, patinaRepo, diskImageName, setupPagesName)
	if err != nil || tag != "v0.1.0-alpha.5" || sums[diskImageName] != strings.Repeat("a", 64) {
		t.Fatalf("release files: %q %v %v", tag, sums, err)
	}
	var p setupPages
	if err := fetchJSON(ctx, http.DefaultClient, releaseAsset(patinaRepo, tag, setupPagesName), sums[setupPagesName], &p); err != nil || p.Theme != "aurora" {
		t.Errorf("pages: %+v, %v", p, err)
	}
	if err := fetchJSON(ctx, http.DefaultClient, releaseAsset(patinaRepo, tag, setupPagesName), sums[diskImageName], &p); err == nil {
		t.Error("a file with another checksum is accepted")
	}
	// A release without the disk image is one without the files asked for.
	if _, _, err := releaseFiles(ctx, http.DefaultClient, patinaRepo, diskImageName, isoName); !errors.Is(err, errNoImage) {
		t.Errorf("a missing file: %v", err)
	}
}

func TestParts(t *testing.T) {
	dir := t.TempDir()
	whole := []byte("one part, then another")
	sum := sha256.Sum256(whole)
	sums := map[string]string{"disk": hex.EncodeToString(sum[:]), "disk.001": "x", "disk.002": "y", "disk.004": "z"}
	names := parts(sums, "disk")
	if len(names) != 2 || names[1] != "disk.002" {
		t.Fatalf("parts: %v", names) // .004 without .003 is not one of them
	}
	if len(parts(map[string]string{"disk": "x"}, "disk")) != 0 {
		t.Error("a file in one piece has parts")
	}
	var paths []string
	for i, piece := range [][]byte{whole[:9], whole[9:]} {
		p := filepath.Join(dir, names[i])
		os.WriteFile(p, piece, 0o644)
		paths = append(paths, p)
	}
	dest := filepath.Join(dir, "disk")
	if err := joinParts(dest, paths, "0000"); err == nil || fileExists(dest) {
		t.Error("parts that join to another file are accepted")
	}
	if err := joinParts(dest, paths, sums["disk"]); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != string(whole) || fileExists(paths[0]) {
		t.Errorf("joined %q, parts left: %v", b, fileExists(paths[0]))
	}
}

func TestImageReleaseFallsBack(t *testing.T) {
	releases(t, `[
		{"tag_name": "v0.1.0-alpha.5", "assets": []},
		{"tag_name": "v0.1.0-alpha.4", "prerelease": true, "assets": [{"name": "SHA256SUMS"}, {"name": "`+isoName+`"}]},
		{"tag_name": "v0.1.0-alpha.3", "assets": [{"name": "SHA256SUMS"}, {"name": "`+isoName+`"}]},
		{"tag_name": "v0.1.0-alpha.2", "assets": []}
	]`)
	sum := sha256.Sum256([]byte(fakeImage))
	tag, got, err := imageRelease(context.Background(), http.DefaultClient, patinaRepo, isoName)
	if err != nil || tag != "v0.1.0-alpha.3" || got != hex.EncodeToString(sum[:]) {
		t.Errorf("imageRelease = %q, %q, %v; want the newest release with an image that is no pre-release", tag, got, err)
	}
}

func TestImageReleaseWithoutAny(t *testing.T) {
	for _, listing := range []string{"", `[{"tag_name": "v0.1.0-alpha.5", "assets": []}]`} {
		releases(t, listing)
		_, _, err := imageRelease(context.Background(), http.DefaultClient, patinaRepo, isoName)
		if !errors.Is(err, errNoImage) || !strings.Contains(err.Error(), "patina v0.1.0-alpha.5 has no image yet") {
			t.Errorf("listing %q: %v; want that the latest has no image yet", listing, err)
		}
	}
}

func TestDownloadParts(t *testing.T) {
	fakeGitHub(t)
	sum := sha256.Sum256([]byte(fakeImage))
	want := hex.EncodeToString(sum[:])
	for _, c := range []struct {
		name, part string
	}{
		{"an older image's part is replaced", strings.Repeat("amethyst ", 30000)},
		{"a part longer than the image is replaced", fakeImage + "and more"},
		{"a complete part is kept", fakeImage},
	} {
		dest := filepath.Join(t.TempDir(), isoName)
		os.WriteFile(dest+".part", []byte(c.part), 0o644)
		err := download(context.Background(), http.DefaultClient,
			releaseAsset(patinaRepo, "v0.1.0-alpha.3", isoName), dest, want, nil)
		if b, _ := os.ReadFile(dest); err != nil || string(b) != fakeImage {
			t.Errorf("%s: %v, %d bytes", c.name, err, len(b))
		}
		if fileExists(dest + ".part") {
			t.Errorf("%s: the part stays", c.name)
		}
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
