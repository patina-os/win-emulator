// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeQEMU speaks just enough QMP: the greeting, a reply per command, and a SHUTDOWN
// event after system_powerdown.
func fakeQEMU(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.Write([]byte(`{"QMP": {"version": {}, "capabilities": []}}` + "\n"))
		s := bufio.NewScanner(c)
		for s.Scan() {
			switch {
			case strings.Contains(s.Text(), "qmp_capabilities"):
				c.Write([]byte(`{"return": {}}` + "\n"))
			case strings.Contains(s.Text(), "system_powerdown"):
				c.Write([]byte(`{"event": "POWERDOWN", "data": {}}` + "\n"))
				c.Write([]byte(`{"return": {}}` + "\n"))
				c.Write([]byte(`{"event": "SHUTDOWN", "data": {"guest": true, "reason": "guest-shutdown"}}` + "\n"))
				return
			default:
				c.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "no such command"}}` + "\n"))
			}
		}
	}()
	return l.Addr().String()
}

func TestQMP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q, err := dialQMP(ctx, fakeQEMU(t))
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if err := q.Execute("nonsense"); err == nil || !strings.Contains(err.Error(), "no such command") {
		t.Errorf("an error reply is an error, got %v", err)
	}
	if err := q.Execute("system_powerdown"); err != nil {
		t.Fatal(err)
	}
	reason := ""
	for e := range q.Events() {
		if e.Event == "SHUTDOWN" {
			reason = shutdownReason(e)
		}
	}
	if reason != "guest-shutdown" {
		t.Errorf("shutdown reason %q, want guest-shutdown", reason)
	}
}

func TestQMPWaitsForQEMU(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := dialQMP(ctx, "127.0.0.1:1"); err == nil {
		t.Error("no QEMU, no connection")
	}
}
