// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
)

// The start page is what an installed Try patina opens with: it starts patina by itself
// after startSeconds, unless the user touches it, to change how large the machine is or
// to install patina again, with its installer, or to remove it, which the tray's menu
// can do only once a machine runs. It is served and shown as the setup window is.

// startSeconds is how long the start page waits before it starts patina by itself.
const startSeconds = 10

// What the start page answers with.
const (
	actionStart     = "start"
	actionReset     = "reset"     // install patina again
	actionInstaller = "installer" // install it with patina's installer
	actionRemove    = "remove"
)

// look is the palette and the fonts of the release the machine was set up from
// (patina-setup.json), kept in lookFile for the start page; without it the page is
// drawn in its own greys.
type look struct {
	Palette map[string]string `json:"palette,omitempty"`
	Fonts   map[string]string `json:"fonts,omitempty"`
}

// startInfo is what the start page shows.
type startInfo struct {
	look
	Version string `json:"version"` // the launcher's
	Release string `json:"release"` // the patina release the machine was set up from
	QEMU    string `json:"qemu"`
	German  bool   `json:"german"`
	Seconds int    `json:"seconds"`
	// What the launcher picks by itself, and what the user may pick instead.
	AutoMemoryMB  int   `json:"auto_memory_mb"`
	AutoCPUs      int   `json:"auto_cpus"`
	MemoryChoices []int `json:"memory_choices"`
	CPUChoices    []int `json:"cpu_choices"`
	// Whether the host's processor can be the guest's here: the hypervisor, and a QEMU
	// new enough (cpuHostMajor).
	HostCPU  bool     `json:"host_cpu"`
	Settings Settings `json:"settings"`
}

// startChoice is what the page sends back: what to do, and the settings to start with.
type startChoice struct {
	Action   string   `json:"action"`
	Settings Settings `json:"settings"`
}

// check says what is wrong with a choice; nil when nothing is. The page offers only
// what passes, so the words are for the log.
func (c startChoice) check(i startInfo) error {
	s := c.Settings
	switch {
	case !slices.Contains([]string{actionStart, actionReset, actionInstaller, actionRemove}, c.Action):
		return errors.New("unknown action")
	case s.MemoryMB != 0 && !slices.Contains(i.MemoryChoices, s.MemoryMB):
		return errors.New("this PC cannot give the machine that much memory")
	case s.CPUs != 0 && !slices.Contains(i.CPUChoices, s.CPUs):
		return errors.New("this PC cannot give the machine that many processors")
	case s.HostCPU && !i.HostCPU:
		return errors.New("the host's processor cannot be the machine's here")
	}
	return nil
}

//go:embed start.html
var startHTML []byte

// startServer serves the start page and takes its choice.
type startServer struct {
	*localPage
	choice chan startChoice
}

// startStart serves the start page. folder is called when the page asks for the data
// folder, which opens beside the page and is no choice.
func startStart(info startInfo, icon []byte, folder func()) (*startServer, error) {
	page, err := newLocalPage(startHTML, icon)
	if err != nil {
		return nil, err
	}
	s := &startServer{localPage: page, choice: make(chan startChoice, 1)}
	s.mux.HandleFunc("GET "+s.prefix+"start.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})
	s.mux.HandleFunc("POST "+s.prefix+"folder", func(w http.ResponseWriter, r *http.Request) { folder() })
	s.mux.HandleFunc("POST "+s.prefix+"choose", func(w http.ResponseWriter, r *http.Request) {
		var c startChoice
		w.Header().Set("Content-Type", "application/json")
		err := json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&c)
		if err == nil {
			err = c.check(info)
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		select {
		case s.choice <- c:
		default: // chosen already
		}
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	return s, nil
}
