// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// QMP is a connection to QEMU's machine protocol: JSON lines, one command at a time,
// events in between. https://www.qemu.org/docs/master/interop/qmp-spec.html
type QMP struct {
	conn    net.Conn
	mu      sync.Mutex // one command in flight
	id      int        // of the last command; QEMU repeats it in the reply
	replies chan qmpMessage
	events  chan qmpMessage
	closed  chan struct{} // closed when QEMU closes the connection
}

type qmpMessage struct {
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
	Return json.RawMessage `json:"return"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
	QMP json.RawMessage `json:"QMP"`
	ID  int             `json:"id"`
}

// dialQMP connects once QEMU listens (it starts listening a moment after it starts)
// and leaves the greeting's capability negotiation done.
func dialQMP(ctx context.Context, addr string) (*QMP, error) {
	var d net.Dialer
	var conn net.Conn
	var err error
	for {
		conn, err = d.DialContext(ctx, "tcp", addr)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("QEMU's machine protocol on %s: %w", addr, err)
		case <-time.After(200 * time.Millisecond):
		}
	}
	q := &QMP{conn: conn, replies: make(chan qmpMessage, 4), events: make(chan qmpMessage, 16),
		closed: make(chan struct{})}
	greeting := make(chan error, 1)
	go q.read(greeting)
	select {
	case err := <-greeting:
		if err != nil {
			conn.Close()
			return nil, err
		}
	case <-ctx.Done():
		conn.Close()
		return nil, ctx.Err()
	}
	if err := q.Execute("qmp_capabilities"); err != nil {
		conn.Close()
		return nil, err
	}
	return q, nil
}

// read hands replies to Execute and events to Events until the connection closes,
// which closes the events channel.
func (q *QMP) read(greeting chan<- error) {
	defer close(q.events)
	defer close(q.closed)
	s := bufio.NewScanner(q.conn)
	s.Buffer(make([]byte, 64<<10), 16<<20)
	first := true
	for s.Scan() {
		var m qmpMessage
		if err := json.Unmarshal(s.Bytes(), &m); err != nil {
			continue
		}
		switch {
		case first:
			first = false
			if m.QMP == nil {
				greeting <- errors.New("QEMU's machine protocol sent no greeting")
				return
			}
			greeting <- nil
		case m.Event != "":
			q.events <- m // the supervisor reads every event; SHUTDOWN must not get lost
		default:
			select {
			case q.replies <- m:
			default: // replies to commands that timed out, which no one waits for
			}
		}
	}
	if first {
		greeting <- errors.New("QEMU's machine protocol closed before its greeting")
	}
}

// Execute runs one command without arguments and waits for its reply. It returns at
// once when QEMU closes the connection, as it does right after quit.
func (q *QMP) Execute(command string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.id++
	b, _ := json.Marshal(map[string]any{"execute": command, "id": q.id})
	q.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := q.conn.Write(append(b, '\n')); err != nil {
		return err
	}
	result := func(m qmpMessage) error {
		if m.Error != nil {
			return fmt.Errorf("%s: %s", command, m.Error.Desc)
		}
		return nil
	}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m := <-q.replies:
			if m.ID == q.id {
				return result(m)
			} // else a late reply to a command that timed out
		case <-q.closed:
			// The reply may have come just before; read has queued all it got.
			for len(q.replies) > 0 {
				if m := <-q.replies; m.ID == q.id {
					return result(m)
				}
			}
			return fmt.Errorf("%s: QEMU closed its machine protocol", command)
		case <-timeout:
			return fmt.Errorf("%s: no reply from QEMU", command)
		}
	}
}

// Events is closed when QEMU closes the connection, that is, when it exits.
func (q *QMP) Events() <-chan qmpMessage { return q.events }

func (q *QMP) Close() error { return q.conn.Close() }

// shutdownReason is the reason of a SHUTDOWN event: guest-shutdown (patina powered
// off), guest-reset (it restarted, under -no-reboot), host-qmp-quit (the tray's force
// off) and others.
func shutdownReason(m qmpMessage) string {
	var d struct {
		Reason string `json:"reason"`
	}
	json.Unmarshal(m.Data, &d)
	return d.Reason
}
