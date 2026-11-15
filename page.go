// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// localPage is a page of the launcher's own, served on 127.0.0.1 under a path no other
// program can guess and shown in a window (window in setup_windows.go): the setup
// window and the start page. Its script says every few seconds that it is open, and
// says so when it is canceled; what it answers is the page's own handler.
type localPage struct {
	URL      string
	prefix   string
	mux      *http.ServeMux
	srv      *http.Server
	canceled chan struct{}
	cancel   sync.Once
	alive    chan struct{}
}

func newLocalPage(html, icon []byte) (*localPage, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &localPage{prefix: "/" + hex.EncodeToString(token) + "/", mux: http.NewServeMux(),
		canceled: make(chan struct{}), alive: make(chan struct{}, 1)}
	p.URL = "http://" + ln.Addr().String() + p.prefix
	p.mux.HandleFunc("GET "+p.prefix+"{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(html)
	})
	p.mux.HandleFunc("GET "+p.prefix+"icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon)
	})
	p.mux.HandleFunc("POST "+p.prefix+"cancel", func(w http.ResponseWriter, r *http.Request) {
		p.cancel.Do(func() { close(p.canceled) })
	})
	p.mux.HandleFunc("POST "+p.prefix+"alive", func(w http.ResponseWriter, r *http.Request) {
		select {
		case p.alive <- struct{}{}:
		default:
		}
	})
	p.srv = &http.Server{Handler: p.mux, ReadHeaderTimeout: 10 * time.Second}
	go p.srv.Serve(ln)
	return p, nil
}

func (p *localPage) Close() error { return p.srv.Close() }

// errPageClosed is a page closed or canceled without an answer.
var errPageClosed = errors.New("the window was closed")

// waitPage returns the answer once the page sent it. It gives up when the page was
// canceled, when closed reports that its window closed, or when the page, which says it
// is open every few seconds, has gone quiet: a browser tab closed. quiet is how long that
// may take; before the page first speaks it gets five minutes, for a browser to start.
func waitPage[T any](p *localPage, answers <-chan T, closed <-chan struct{}, quiet time.Duration) (T, error) {
	var none T
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case a := <-answers:
			return a, nil
		case <-p.canceled:
			return none, errPageClosed
		case <-closed:
			select { // the last answer may have come with the window's end
			case a := <-answers:
				return a, nil
			default:
				return none, errPageClosed
			}
		case <-p.alive:
			timer.Reset(quiet)
		case <-timer.C:
			return none, errPageClosed
		}
	}
}
