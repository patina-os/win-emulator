// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func testStartInfo() startInfo {
	return startInfo{Version: "v0.0.9", Release: "v0.1.0-alpha.17", QEMU: "11.1.0", Seconds: startSeconds,
		AutoMemoryMB: 8192, AutoCPUs: 4, MemoryChoices: memoryChoices(16384), CPUChoices: cpuChoices(8), HostCPU: true}
}

func TestStartChoiceCheck(t *testing.T) {
	i := testStartInfo()
	for _, c := range []startChoice{
		{Action: actionStart},
		{Action: actionStart, Settings: Settings{MemoryMB: 12288, CPUs: 8, HostCPU: true}},
		{Action: actionReset}, {Action: actionInstaller}, {Action: actionRemove},
	} {
		if err := c.check(i); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
	old := i
	old.HostCPU = false // a QEMU before the host model's fixes, or no hypervisor
	for _, c := range []startChoice{
		{Action: ""}, {Action: "quit"},
		{Action: actionStart, Settings: Settings{MemoryMB: 16384}}, // all of the host's memory
		{Action: actionStart, Settings: Settings{MemoryMB: 5000}},
		{Action: actionStart, Settings: Settings{CPUs: 12}},
		{Action: actionStart, Settings: Settings{CPUs: 1}},
	} {
		if err := c.check(i); err == nil {
			t.Errorf("%+v passes", c)
		}
	}
	if err := (startChoice{Action: actionStart, Settings: Settings{HostCPU: true}}).check(old); err == nil {
		t.Error("the host's processor passes where it is not offered")
	}
}

func TestStartServer(t *testing.T) {
	folders := 0
	s, err := startStart(testStartInfo(), []byte("png"), func() { folders++ })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	resp, err := http.Get(s.URL + "start.json")
	if err != nil {
		t.Fatal(err)
	}
	var served startInfo
	json.NewDecoder(resp.Body).Decode(&served)
	if served.Release != "v0.1.0-alpha.17" || served.Seconds != startSeconds || !slices.Equal(served.CPUChoices, []int{2, 4, 6, 8}) {
		t.Errorf("start.json: %+v", served)
	}
	if resp, err := http.Get(s.URL); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("the page: %v", err)
	}
	post := func(path, body string) int {
		resp, err := http.Post(s.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if post("folder", ""); folders != 1 {
		t.Errorf("the data folder was opened %d times", folders)
	}
	if code := post("choose", `{"action": "start", "settings": {"memory_mb": 99}}`); code != http.StatusBadRequest {
		t.Errorf("a choice this PC cannot give: %d", code)
	}
	if code := post("choose", `{"action": "start", "settings": {"memory_mb": 6144, "cpus": 2, "host_cpu": true}}`); code != http.StatusOK {
		t.Errorf("a good choice: %d", code)
	}
	c, err := waitPage(s.localPage, s.choice, nil, time.Minute)
	if err != nil || c.Action != actionStart || c.Settings != (Settings{MemoryMB: 6144, CPUs: 2, HostCPU: true}) {
		t.Errorf("wait: %+v, %v", c, err)
	}
	http.Post(s.URL+"cancel", "", nil)
	if _, err := waitPage(s.localPage, s.choice, nil, time.Minute); err != errPageClosed {
		t.Errorf("canceled: %v", err)
	}
}
