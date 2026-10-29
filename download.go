// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// github is where releases come from; tests point it at a local server.
var github = "https://github.com"

// latestRelease is the tag of a repository's latest release, read from the redirect
// of /releases/latest, so no API token or rate limit is involved.
func latestRelease(ctx context.Context, c *http.Client, repo string) (string, error) {
	nc := *c
	nc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, github+"/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := nc.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	_, tag, ok := strings.Cut(loc, "/releases/tag/")
	if !ok || tag == "" || strings.Contains(tag, "/") {
		return "", fmt.Errorf("%s has no release yet (%s)", repo, resp.Status)
	}
	return tag, nil
}

func releaseAsset(repo, tag, name string) string {
	return github + "/" + repo + "/releases/download/" + tag + "/" + name
}

// checksum is the SHA-256 that a SHA256SUMS file (sha256sum's output) names for a file.
func checksum(ctx context.Context, c *http.Client, url, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	s := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s names no %s", url, name)
}

// download fetches url to dest and checks its SHA-256. It continues a .part file left
// by an earlier, interrupted run where the server allows. progress gets the bytes so
// far and the total (0 when unknown).
func download(ctx context.Context, c *http.Client, url, dest, want string, progress func(done, total int64)) error {
	part := dest + ".part"
	h := sha256.New()
	var have int64
	if f, err := os.Open(part); err == nil {
		have, _ = io.Copy(h, f)
		f.Close()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		flags |= os.O_TRUNC
		have = 0
		h.Reset()
	case http.StatusRequestedRangeNotSatisfiable:
		// The part is complete already, or longer than the file: start over.
		os.Remove(part)
		return download(ctx, c, url, dest, want, progress)
	default:
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	total := int64(0)
	if resp.ContentLength > 0 {
		total = have + resp.ContentLength
	}
	w := &progressWriter{done: have, total: total, report: progress}
	_, err = io.Copy(io.MultiWriter(f, h, w), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.Remove(part)
		return fmt.Errorf("%s does not match its checksum (got %s, want %s); it was deleted", url, got, want)
	}
	return os.Rename(part, dest)
}

type progressWriter struct {
	done, total int64
	report      func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.report != nil {
		p.report(p.done, p.total)
	}
	return len(b), nil
}

// fileSHA256 is the checksum of a file already on disk.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
