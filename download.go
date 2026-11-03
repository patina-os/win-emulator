// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// github is where releases come from, and githubAPI lists them when the latest has no
// image; tests point both at a local server.
var (
	github    = "https://github.com"
	githubAPI = "https://api.github.com"
)

// errNoImage is a release without the image or without its checksum.
var errNoImage = errors.New("has no image yet")

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

// checksums are the SHA-256 sums a SHA256SUMS file (sha256sum's output) names, by file.
func checksums(ctx context.Context, c *http.Client, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %s: %w", url, resp.Status, errNoImage)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	sums := map[string]string{}
	s := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) == 2 && len(f[0]) == 64 {
			sums[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
		}
	}
	return sums, s.Err()
}

// releaseSums are the checksums of a release's files, as long as it has every one of
// names; errNoImage when it lacks one.
func releaseSums(ctx context.Context, c *http.Client, repo, tag string, names []string) (map[string]string, error) {
	url := releaseAsset(repo, tag, sumsName)
	sums, err := checksums(ctx, c, url)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if sums[name] == "" {
			return nil, fmt.Errorf("%s names no %s: %w", url, name, errNoImage)
		}
	}
	return sums, nil
}

// imageRelease is the release to install from and its image's checksum (see
// releaseFiles).
func imageRelease(ctx context.Context, c *http.Client, repo, image string) (tag, sum string, err error) {
	tag, sums, err := releaseFiles(ctx, c, repo, image)
	return tag, sums[image], err
}

// releaseFiles is the release to install from, one that has all of names, and the
// checksums of its files: the latest release, or, while that lacks one of them, the
// newest release before it that has them all. patina publishes a release first and
// attaches its images once they passed their boot test, a quarter of an hour later or,
// when the test fails, not at all.
func releaseFiles(ctx context.Context, c *http.Client, repo string, names ...string) (tag string, sums map[string]string, err error) {
	latest, err := latestRelease(ctx, c, repo)
	if err != nil {
		return "", nil, err
	}
	sums, err = releaseSums(ctx, c, repo, latest, names)
	if !errors.Is(err, errNoImage) {
		return latest, sums, err
	}
	// GitHub's API, which needs no token for this one request, lists the releases
	// newest first; /releases/latest skips drafts and pre-releases, and so does this.
	noImage := fmt.Errorf("patina %s %w. Try again in an hour", latest, errNoImage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI+"/repos/"+repo+"/releases?per_page=20", nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Do(req)
	if err != nil {
		return "", nil, noImage
	}
	defer resp.Body.Close()
	var releases []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&releases) != nil {
		return "", nil, noImage
	}
	for _, r := range releases {
		has := map[string]bool{}
		for _, a := range r.Assets {
			has[a.Name] = true
		}
		complete := !r.Draft && !r.Prerelease && r.Tag != latest && has[sumsName]
		for _, name := range names {
			complete = complete && (has[name] || has[name+".001"]) // in one piece or in parts
		}
		if !complete {
			continue
		}
		if sums, err := releaseSums(ctx, c, repo, r.Tag, names); err == nil {
			return r.Tag, sums, nil
		}
	}
	return "", nil, noImage
}

// download fetches url to dest and checks its SHA-256. It continues a .part file left
// by an earlier, interrupted run where the server allows, and starts over once when
// the part turns out to be of another file (an older image of the same name).
// progress gets the bytes so far and the total (0 when unknown).
func download(ctx context.Context, c *http.Client, url, dest, want string, progress func(done, total int64)) error {
	part := dest + ".part"
	h := sha256.New()
	var have int64
	if f, err := os.Open(part); err == nil {
		have, _ = io.Copy(h, f)
		f.Close()
	}
	if have > 0 && hex.EncodeToString(h.Sum(nil)) == want {
		return os.Rename(part, dest) // complete already: the last run stopped before this
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
		if !strings.HasPrefix(resp.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", have)) {
			resp.Body.Close() // not where the part ends: start over
			os.Remove(part)
			return download(ctx, c, url, dest, want, progress)
		}
		flags |= os.O_APPEND
	case http.StatusOK:
		flags |= os.O_TRUNC
		have = 0
		h.Reset()
	case http.StatusRequestedRangeNotSatisfiable:
		// The part is longer than the file: start over.
		resp.Body.Close()
		os.Remove(part)
		return download(ctx, c, url, dest, want, progress)
	default:
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	resumed := have > 0
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
		if resumed {
			return download(ctx, c, url, dest, want, progress) // this time from the start
		}
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

// fetchJSON fetches a small file whose SHA-256 is want and decodes it into v.
func fetchJSON(ctx context.Context, c *http.Client, url, want string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if got := sha256.Sum256(b); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("%s does not match its checksum", url)
	}
	return json.Unmarshal(b, v)
}

// parts are the names of a file published in parts, as SHA256SUMS lists them: name.001,
// name.002 and on, which joined are the file. GitHub takes no release file of 2 GiB or
// more, so patina publishes its disk image this way. None for a file in one piece.
func parts(sums map[string]string, name string) []string {
	var list []string
	for i := 1; ; i++ {
		part := fmt.Sprintf("%s.%03d", name, i)
		if sums[part] == "" {
			return list
		}
		list = append(list, part)
	}
}

// joinParts writes the files of parts, in order, to dest and checks the result against
// want; the parts are deleted once dest is complete.
func joinParts(dest string, parts []string, want string) error {
	part := dest + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	h := sha256.New()
	for _, p := range parts {
		in, err := os.Open(p)
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(io.MultiWriter(out, h), in)
		in.Close()
		if err != nil {
			out.Close()
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.Remove(part)
		return fmt.Errorf("%s does not match its checksum once joined (got %s, want %s)", filepath.Base(dest), got, want)
	}
	if err := os.Rename(part, dest); err != nil {
		return err
	}
	for _, p := range parts {
		os.Remove(p)
	}
	return nil
}
