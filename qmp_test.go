// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeQEMU speaks just enough QMP: the greeting, a reply per command with its id, a
// stray reply to a command never sent, a SHUTDOWN event after system_powerdown, and
// after quit the event and a closed connection without a reply.
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
			var cmd struct {
				Execute string `json:"execute"`
				ID      int    `json:"id"`
			}
			json.Unmarshal(s.Bytes(), &cmd)
			id := fmt.Sprint(cmd.ID)
			switch cmd.Execute {
			case "qmp_capabilities":
				c.Write([]byte(`{"return": {}, "id": ` + id + `}` + "\n"))
			case "system_powerdown":
				c.Write([]byte(`{"event": "POWERDOWN", "data": {}}` + "\n"))
				c.Write([]byte(`{"return": {}, "id": 999}` + "\n"))
				c.Write([]byte(`{"return": {}, "id": ` + id + `}` + "\n"))
				c.Write([]byte(`{"event": "SHUTDOWN", "data": {"guest": true, "reason": "guest-shutdown"}}` + "\n"))
				return
			case "quit":
				c.Write([]byte(`{"event": "SHUTDOWN", "data": {"guest": false, "reason": "host-qmp-quit"}}` + "\n"))
				return
			default:
				c.Write([]byte(`{"error": {"class": "CommandNotFound", "desc": "no such command"}, "id": ` + id + `}` + "\n"))
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

func TestQMPQuitReturnsWhenQEMUCloses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q, err := dialQMP(ctx, fakeQEMU(t))
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	go func() {
		for range q.Events() {
		}
	}()
	start := time.Now()
	q.Execute("quit") // an error: QEMU closed without a reply
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("quit took %v; Execute must return when QEMU closes the connection", d)
	}
}

func TestQMPWaitsForQEMU(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := dialQMP(ctx, "127.0.0.1:1"); err == nil {
		t.Error("no QEMU, no connection")
	}
}
