// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// testPages are pages as patina-setup.json has them, cut down.
func testPages(t *testing.T) setupPages {
	var p setupPages
	err := json.Unmarshal([]byte(`{"theme": "aurora", "palette": {"accent": "#3cff9d"}, "pages": [
	  {"id": "agent", "key": "PATINA_AI_AGENT", "mode": "required", "default": "none",
	   "labels": {"step": "AI agent", "step[de]": "KI-Agent"},
	   "items": [{"id": "opencode", "name": "opencode"}, {"id": "none", "name": "No AI agent"}]},
	  {"id": "ai", "key": "PATINA_AI", "mode": "required", "default": "local", "labels": {"step": "AI model"},
	   "items": [{"id": "local"}, {"id": "cloud"}]},
	  {"id": "apps", "key": "PATINA_BUNDLES", "mode": "required", "default": "desktop", "labels": {"step": "Apps"},
	   "items": [{"id": "desktop"}, {"id": "minimal"}]},
	  {"id": "more", "key": "PATINA_BUNDLES", "mode": "optionalmultiple", "labels": {"step": "More apps"},
	   "items": [{"id": ""}, {"id": "headstart"}, {"id": "artcraft"}]},
	  {"id": "theme", "key": "PATINA_THEME", "mode": "required", "default": "aurora", "labels": {"step": "Theme"},
	   "items": [{"id": "aurora"}, {"id": "datum"}]}]}`), &p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func testAnswers() setupAnswers {
	return setupAnswers{FullName: "Ada Lovelace", User: "ada", Password: "correct horse", Locale: "de_DE.UTF-8",
		Keyboard: "de(nodeadkeys)", TimeZone: "Europe/Berlin", Choices: map[string][]string{
			"agent": {"opencode"}, "ai": {"cloud"}, "apps": {"desktop"}, "more": {"headstart", "artcraft"},
			"theme": {"datum"}}}
}

func TestSetupEnv(t *testing.T) {
	p := testPages(t)
	a := testAnswers()
	if err := a.check(p); err != nil {
		t.Fatal(err)
	}
	want := `PATINA_USER=ada
PATINA_FULLNAME=Ada Lovelace
PATINA_PASSWORD=correct horse
PATINA_LOCALE=de_DE.UTF-8
PATINA_KEYBOARD_LAYOUT=de
PATINA_KEYBOARD_VARIANT=nodeadkeys
PATINA_TIMEZONE=Europe/Berlin
PATINA_AI_AGENT=opencode
PATINA_AI=cloud
PATINA_BUNDLES=desktop,headstart,artcraft
PATINA_THEME=datum
`
	if got := a.env(p); got != want {
		t.Errorf("env:\n%s\nwant:\n%s", got, want)
	}
	// Nothing more picked: the bundles are the apps page's alone; a layout without variant.
	a.Choices["more"] = nil
	a.Keyboard = "us"
	env := a.env(p)
	if !strings.Contains(env, "\nPATINA_BUNDLES=desktop\n") || !strings.Contains(env, "\nPATINA_KEYBOARD_VARIANT=\n") ||
		!strings.Contains(env, "\nPATINA_KEYBOARD_LAYOUT=us\n") {
		t.Errorf("env without more apps:\n%s", env)
	}
}

func TestSetupCheck(t *testing.T) {
	p := testPages(t)
	for name, change := range map[string]func(*setupAnswers){
		"root":            func(a *setupAnswers) { a.User = "root" },
		"capitals":        func(a *setupAnswers) { a.User = "Ada" },
		"short password":  func(a *setupAnswers) { a.Password = "short" },
		"newline":         func(a *setupAnswers) { a.Password = "line one\nPATINA_USER=root" },
		"newline in name": func(a *setupAnswers) { a.FullName = "Ada\nPATINA_THEME=x" },
		"locale":          func(a *setupAnswers) { a.Locale = "de_DE.UTF-8\nX" },
		"keyboard":        func(a *setupAnswers) { a.Keyboard = "de(no dead keys)" },
		"time zone":       func(a *setupAnswers) { a.TimeZone = "../../etc/passwd" },
		"two themes":      func(a *setupAnswers) { a.Choices["theme"] = []string{"aurora", "datum"} },
		"no apps":         func(a *setupAnswers) { delete(a.Choices, "apps") },
		"unknown item":    func(a *setupAnswers) { a.Choices["more"] = []string{"games"} },
		"twice":           func(a *setupAnswers) { a.Choices["more"] = []string{"headstart", "headstart"} },
		"unknown page":    func(a *setupAnswers) { a.Choices["games"] = []string{"chess"} },
	} {
		a := testAnswers()
		change(&a)
		if a.check(p) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSetupServer(t *testing.T) {
	p := testPages(t)
	s, err := startSetup(p, setupDefaults{User: "ada"}, []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if resp, err := http.Get(strings.TrimSuffix(s.URL, "/") + "x/setup.json"); err != nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("another path is served: %v", err)
	}
	resp, err := http.Get(s.URL + "setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var served struct {
		Pages    []setupPage   `json:"pages"`
		Defaults setupDefaults `json:"defaults"`
	}
	json.NewDecoder(resp.Body).Decode(&served)
	if len(served.Pages) != 5 || served.Defaults.User != "ada" {
		t.Errorf("setup.json: %+v", served)
	}
	post := func(a setupAnswers) int {
		b, _ := json.Marshal(a)
		resp, err := http.Post(s.URL+"submit", "application/json", strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	bad := testAnswers()
	bad.User = "root"
	if code := post(bad); code != http.StatusBadRequest {
		t.Errorf("a bad answer: %d", code)
	}
	if code := post(testAnswers()); code != http.StatusOK {
		t.Errorf("a good answer: %d", code)
	}
	a, err := s.wait(nil, time.Minute)
	if err != nil || a.User != "ada" {
		t.Errorf("wait: %+v, %v", a, err)
	}
}

func TestSetupWaitGivesUp(t *testing.T) {
	p := testPages(t)
	s, err := startSetup(p, setupDefaults{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// A page that spoke once and went quiet: its tab was closed.
	http.Post(s.URL+"alive", "", nil)
	if _, err := s.wait(nil, 100*time.Millisecond); err != errSetupCanceled {
		t.Errorf("a quiet page: %v", err)
	}
	// Its window closed.
	closed := make(chan struct{})
	close(closed)
	if _, err := s.wait(closed, time.Minute); err != errSetupCanceled {
		t.Errorf("a closed window: %v", err)
	}
	// Canceled.
	http.Post(s.URL+"cancel", "", nil)
	if _, err := s.wait(nil, time.Minute); err != errSetupCanceled {
		t.Errorf("canceled: %v", err)
	}
}

func TestLocaleFromWindows(t *testing.T) {
	for in, want := range map[string]string{"de-DE": "de_DE.UTF-8", "en-GB": "en_GB.UTF-8",
		"sr-Latn-RS": "sr_RS.UTF-8", "fil-PH": "fil_PH.UTF-8", "de": "en_GB.UTF-8", "": "en_GB.UTF-8",
		"x-y": "en_GB.UTF-8"} {
		if got := localeFor(in); got != want {
			t.Errorf("localeFor(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"00000407": "de", "00020409": "us(intl)", "0000040c": "fr",
		"E0200804": "us", "00010415": "pl(qwertz)"} {
		if got := keyboardFor(in); got != want {
			t.Errorf("keyboardFor(%q) = %q, want %q", in, got, want)
		}
	}
	seen := map[string]bool{}
	for _, k := range keyboardChoices() {
		if seen[k.ID] || !keyboardPattern.MatchString(k.ID) {
			t.Errorf("keyboard %q listed twice or malformed", k.ID)
		}
		seen[k.ID] = true
	}
	for _, l := range locales {
		if !localePattern.MatchString(l.ID) {
			t.Errorf("locale %q malformed", l.ID)
		}
	}
	for _, z := range append(timeZones, "Europe/Berlin") {
		if !timeZonePattern.MatchString(z) {
			t.Errorf("time zone %q malformed", z)
		}
	}
	if got := localeChoices("eo_XX.UTF-8"); got[0].ID != "eo_XX.UTF-8" || len(got) != len(locales)+1 {
		t.Errorf("Windows' own language is not listed first")
	}
	for in, want := range map[string]string{`WORK\Ada.Lovelace`: "adalovelace", "Jürgen": "jrgen",
		"123": "", "Administrator": "administrator", "root": ""} {
		if got := loginName(in); got != want {
			t.Errorf("loginName(%q) = %q, want %q", in, got, want)
		}
	}
}
