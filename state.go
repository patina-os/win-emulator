// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// The phases of the virtual machine's life, kept in state.json in the data directory.
const (
	phaseNew        = ""           // nothing set up yet, or reset
	phaseInstalling = "installing" // the disk exists, the installer has not finished
	phaseInstalled  = "installed"  // patina boots from the disk
)

type State struct {
	Phase   string `json:"phase"`
	Release string `json:"release,omitempty"` // the patina release it was installed from
}

// Files in the data directory.
const (
	stateFile = "state.json"
	diskFile  = "disk.qcow2"
	varsFile  = "efivars.fd"
	iconFile  = "patina.ico"
	qemuLog   = "qemu.log"
	appLog    = "launcher.log"
)

func loadState(dir string) (State, error) {
	var s State
	b, err := os.ReadFile(filepath.Join(dir, stateFile))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

// save writes the state through a temporary file, so a crash never leaves half a file.
func (s State) save(dir string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stateFile))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
