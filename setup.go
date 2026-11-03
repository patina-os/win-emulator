// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// The setup window asks on Windows what patina's installer asks on a PC: the account,
// the language, keyboard and time zone (taken from Windows, so they can be changed),
// and the installer's own pages, which patina publishes as patina-setup.json next to
// the disk image (iso/disk/choices.sh in patina). The answers reach the guest once, as
// a KEY=value file QEMU hands over (setupFwCfg; patina's
// devices/try-patina-windows/README.md has the keys). The page is served on 127.0.0.1
// under a random path and shown in an Edge app window (setup_windows.go).

// setupPages is patina-setup.json: the default theme's palette and the fonts the page
// is drawn in, and the installer's pages as Calamares has them.
type setupPages struct {
	Theme   string            `json:"theme"`
	Palette map[string]string `json:"palette"`
	Fonts   map[string]string `json:"fonts"`
	Pages   []setupPage       `json:"pages"`
}

type setupPage struct {
	ID      string `json:"id"`
	Key     string `json:"key"`  // the first boot's variable the answer goes to
	Mode    string `json:"mode"` // required (one item) or optionalmultiple
	Default string `json:"default"`
	Labels  struct {
		Step   string `json:"step"`
		StepDE string `json:"step[de]"`
	} `json:"labels"`
	Items []setupItem `json:"items"`
}

type setupItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	NameDE        string `json:"name[de]"`
	Description   string `json:"description"`
	DescriptionDE string `json:"description[de]"`
	// The note a page of boxes to tick shows in place of its empty item's description,
	// which tells the installer's users to hold Ctrl for more than one.
	Note    string `json:"note,omitempty"`
	NoteDE  string `json:"note[de],omitempty"`
	Picture string `json:"picture"` // a data: URL, shown for the themes
}

// setupDefaults is what the page starts with: Windows' language, keyboard and time
// zone and the Windows user's name, and the lists to change them from.
type setupDefaults struct {
	FullName  string   `json:"fullname"`
	User      string   `json:"user"`
	Locale    string   `json:"locale"`
	Keyboard  string   `json:"keyboard"`
	TimeZone  string   `json:"timezone"`
	German    bool     `json:"german"`
	Locales   []choice `json:"locales"`
	Keyboards []choice `json:"keyboards"`
	TimeZones []string `json:"timezones"`
}

type choice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// setupAnswers is what the page sends back.
type setupAnswers struct {
	FullName string              `json:"fullname"`
	User     string              `json:"user"`
	Password string              `json:"password"`
	Locale   string              `json:"locale"`
	Keyboard string              `json:"keyboard"` // an XKB layout, or layout(variant)
	TimeZone string              `json:"timezone"`
	Choices  map[string][]string `json:"choices"` // page id: the item ids picked
}

var (
	// The names patina's installer accepts for an account, root and live aside.
	userPattern     = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	localePattern   = regexp.MustCompile(`^[a-z]{2,3}(_[A-Z]{2})?\.UTF-8$`)
	keyboardPattern = regexp.MustCompile(`^([a-z]{2,})(\(([a-z0-9_]+)\))?$`)
	timeZonePattern = regexp.MustCompile(`^[A-Za-z0-9_+-]+(/[A-Za-z0-9_+-]+)*$`)
)

// minPassword is the installer's shortest password (users.conf in patina).
const minPassword = 8

// check says what is wrong with the answers, in words for the page; nil when nothing is.
func (a setupAnswers) check(p setupPages) error {
	switch {
	case !userPattern.MatchString(a.User) || a.User == "root" || a.User == "live":
		return errors.New("The login name takes small letters, digits, - and _, and starts with a letter.")
	case len([]rune(a.Password)) < minPassword:
		return fmt.Errorf("The password needs at least %d characters.", minPassword)
	case strings.ContainsAny(a.Password+a.FullName, "\r\n"):
		return errors.New("The name and the password fit on one line.")
	case !localePattern.MatchString(a.Locale):
		return errors.New("Pick a language from the list.")
	case !keyboardPattern.MatchString(a.Keyboard):
		return errors.New("Pick a keyboard from the list.")
	case !timeZonePattern.MatchString(a.TimeZone):
		return errors.New("The time zone is a name such as Europe/Berlin.")
	}
	for id := range a.Choices {
		if !slices.ContainsFunc(p.Pages, func(pg setupPage) bool { return pg.ID == id }) {
			return fmt.Errorf("patina has no page %q.", id)
		}
	}
	for _, pg := range p.Pages {
		picked := a.Choices[pg.ID]
		if pg.Mode != "optionalmultiple" && len(picked) != 1 {
			return fmt.Errorf("Pick one answer under %s.", pg.Labels.Step)
		}
		for i, id := range picked {
			if !slices.ContainsFunc(pg.Items, func(it setupItem) bool { return it.ID == id }) ||
				slices.Contains(picked[:i], id) {
				return fmt.Errorf("%q is not an answer under %s.", id, pg.Labels.Step)
			}
		}
	}
	return nil
}

// env is the file the guest reads: one KEY=value per line. The pages' answers go to their
// keys, those of pages sharing a key joined with commas in page order, as the installer
// writes them.
func (a setupAnswers) env(p setupPages) string {
	layout, variant, _ := strings.Cut(strings.TrimSuffix(a.Keyboard, ")"), "(")
	lines := []string{
		"PATINA_USER=" + a.User,
		"PATINA_FULLNAME=" + a.FullName,
		"PATINA_PASSWORD=" + a.Password,
		"PATINA_LOCALE=" + a.Locale,
		"PATINA_KEYBOARD_LAYOUT=" + layout,
		"PATINA_KEYBOARD_VARIANT=" + variant,
		"PATINA_TIMEZONE=" + a.TimeZone,
	}
	var keys []string
	values := map[string][]string{}
	for _, pg := range p.Pages {
		if !slices.Contains(keys, pg.Key) {
			keys = append(keys, pg.Key)
		}
		for _, id := range a.Choices[pg.ID] {
			if id != "" {
				values[pg.Key] = append(values[pg.Key], id)
			}
		}
	}
	for _, k := range keys {
		if len(values[k]) > 0 {
			lines = append(lines, k+"="+strings.Join(values[k], ","))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

//go:embed setup.html
var setupHTML []byte

// setupServer serves the page and takes its answers.
type setupServer struct {
	URL      string
	pages    setupPages
	defaults setupDefaults
	icon     []byte
	srv      *http.Server
	answers  chan setupAnswers
	canceled chan struct{}
	cancel   sync.Once
	alive    chan struct{}
}

// startSetup serves the page on 127.0.0.1, under a path no other program can guess.
func startSetup(p setupPages, d setupDefaults, icon []byte) (*setupServer, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	prefix := "/" + hex.EncodeToString(token) + "/"
	s := &setupServer{URL: "http://" + ln.Addr().String() + prefix, pages: p, defaults: d, icon: icon,
		answers: make(chan setupAnswers, 1), canceled: make(chan struct{}), alive: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+prefix+"{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(setupHTML)
	})
	mux.HandleFunc("GET "+prefix+"setup.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			setupPages
			Defaults setupDefaults `json:"defaults"`
		}{s.pages, s.defaults})
	})
	mux.HandleFunc("GET "+prefix+"icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(s.icon)
	})
	mux.HandleFunc("POST "+prefix+"submit", func(w http.ResponseWriter, r *http.Request) {
		var a setupAnswers
		w.Header().Set("Content-Type", "application/json")
		err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&a)
		if err == nil {
			err = a.check(s.pages)
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		select {
		case s.answers <- a:
		default: // answered already
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST "+prefix+"cancel", func(w http.ResponseWriter, r *http.Request) {
		s.cancel.Do(func() { close(s.canceled) })
	})
	mux.HandleFunc("POST "+prefix+"alive", func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.alive <- struct{}{}:
		default:
		}
	})
	s.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go s.srv.Serve(ln)
	return s, nil
}

// errSetupCanceled is a setup window closed or canceled without answers.
var errSetupCanceled = errors.New("the setup was canceled")

// wait returns the answers once the page sent them. It gives up when the page was
// canceled, when closed reports that its window closed, or when the page, which says it
// is open every few seconds, has gone quiet: a browser tab closed. quiet is how long that
// may take; before the page first speaks it gets five minutes, for a browser to start.
func (s *setupServer) wait(closed <-chan struct{}, quiet time.Duration) (setupAnswers, error) {
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case a := <-s.answers:
			return a, nil
		case <-s.canceled:
			return setupAnswers{}, errSetupCanceled
		case <-closed:
			select { // the last answer may have come with the window's end
			case a := <-s.answers:
				return a, nil
			default:
				return setupAnswers{}, errSetupCanceled
			}
		case <-s.alive:
			timer.Reset(quiet)
		case <-timer.C:
			return setupAnswers{}, errSetupCanceled
		}
	}
}

func (s *setupServer) Close() error { return s.srv.Close() }
