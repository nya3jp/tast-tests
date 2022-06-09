// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Event defines functions common for all hostapd events.
type Event interface {
	ToLogString() string
}

// ApStaConnectedEvent defines data of AP-STA-CONNECTED event.
type ApStaConnectedEvent struct {
	// Addr is the MAC address of the station connected to the AP.
	Addr string
}

// ToLogString format the event data to a string suitable for logging
func (e *ApStaConnectedEvent) ToLogString() string {
	return fmt.Sprintf("%+v\n", e)
}

// Monitor holds the internal context of HostAP daemon monitor.
type Monitor struct {
	cmd           *testexec.Cmd
	stdin         io.WriteCloser
	stdoutScanner *bufio.Scanner
	lines         chan string
}

type eventDef struct {
	matcher   *regexp.Regexp
	parseFunc func(matches []string) (Event, error)
}

var eventDefs = []eventDef{
	{
		regexp.MustCompile(`AP-STA-CONNECTED ([\da-fA-F:]+)`),
		func(matches []string) (Event, error) {
			event := new(ApStaConnectedEvent)
			event.Addr = matches[1]
			return event, nil
		},
	},
}

// NewMonitor provides a reference to a new instance of monitor.
func NewMonitor() *Monitor {
	return &Monitor{}
}

// Start the hostapd monitor connecting hostapd_cli to the running hostapd
// instance.  A goroutine is created to collect hostapd_cli output
// asynchronously. Both the process and the goroutine will need to be stopped
// with a call to m.Stop().
func (m *Monitor) Start(ctx context.Context, s *Server) error {
	cmd := testexec.CommandContext(ctx, "hostapd_cli", "-p", s.ctrlSocketPath, "-i", s.iface)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe to hostapd_cli")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe to hostapd_cli")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start hostapd_cli")
	}

	m.lines = make(chan string, 100)
	m.stdin = stdin
	m.stdoutScanner = bufio.NewScanner(stdout)
	m.cmd = cmd

	go func() {
		defer close(m.lines)
		for m.stdoutScanner.Scan() {
			line := m.stdoutScanner.Text()
			m.lines <- line
		}
	}()

	if err := m.waitReady(ctx); err != nil {
		m.Stop(ctx)
		return err
	}

	return nil
}

// waitReady waits for hostapd_cli to connect to hostapd.
func (m *Monitor) waitReady(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return errors.New("timeout waiting for hostapd_cli interactive mode start")
		case line := <-m.lines:
			if strings.Contains(line, "Interactive mode") {
				return nil
			}
		}
	}
}

// Stop quits hostapd_cli gracefully then wait for both the output processing
// goroutine and the process to end.
func (m *Monitor) Stop(ctx context.Context) error {
	if m.stdin == nil || m.cmd == nil {
		return errors.New("Hostapd monitor not started")
	}

	// Ask hostapd_cli to quit.
	if _, err := io.WriteString(m.stdin, "quit\n"); err != nil {
		testing.ContextLog(ctx, "Failed to send 'quit' command to hostapd_cli: ", err)
	}
	m.stdin.Close()

	// Drain m.lines in case scan goroutine is stuck on writing to a full channel
	// and wait until it's closed there.
	for range m.lines {
	}

	if err := m.cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to wait for hostapd_cli exit")
	}
	return nil
}

// WaitForEvent blocks until an event defined in @eventDefs is emitted or the
// context deadline exceeded.  It includes events already buffered in since the
// last call to WaitForEvent.
// In case of successful match the event is returned; in case of parsing error
// or context deadline exceeded and error is returned.
func (m *Monitor) WaitForEvent(ctx context.Context) (HostapdEvent, error) {
	// hostapd_cli is reluctant to output events and may wait for program
	// termination to print. To avoid it, send the 'ping' command regularly
	// which triggers a PONG reply.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for event")
		case <-ticker.C:
			if _, err := io.WriteString(m.stdin, "ping\n"); err != nil {
				return nil, errors.Wrap(err, "failed to write 'ping' command to hostapd_cli")
			}
		case line := <-m.lines:
			if line == "PONG" {
				// Reply to the 'ping' command triggered by the ticker, just
				// ignore it.
				continue
			}
			for _, eventDef := range eventDefs {
				if matches := eventDef.matcher.FindStringSubmatch(line); matches != nil {
					event, err := eventDef.parseFunc(matches)
					if err != nil {
						return nil, errors.Wrapf(err, "error parsing line: %s", line)
					}
					return event, nil
				}
			}

		}
	}
}
