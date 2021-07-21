// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const debugWPAMonitor = false

// SupplicantEvent defines functions common for all wpa_supplicant events.
type SupplicantEvent interface {
	ToLogString() string
}

// RoamEvent defines data of CTRL-EVENT-DO-ROAM and CTRL-EVENT-SKIP-ROAM events.
type RoamEvent struct {
	CurBSSID         string
	CurFreq          int
	CurLevel         int
	CurEstThroughput int
	SelBSSID         string
	SelFreq          int
	SelLevel         int
	SelEstThroughput int
	Skip             bool
}

// DisconnectedEvent defines data of CTRL-EVENT-DISCONNECTED event.
type DisconnectedEvent struct {
	BSSID            string
	Reason           int
	LocallyGenerated string
	RcvTime          time.Time
}

// ScanResultsEvent defines data of CTRL-EVENT-SCAN-RESULTS event.
type ScanResultsEvent struct {
}

// WPAMonitor holds internal context of the WPA monitor.
type WPAMonitor struct {
	stdin         io.WriteCloser
	stdoutScanner *bufio.Scanner
	cmd           *ssh.Cmd
	lines         chan string
}

type eventDef struct {
	matcher   *regexp.Regexp
	parseFunc func(matches []string) (event SupplicantEvent, firstError error)
}

func parseRoamEvent(matches []string) (event RoamEvent, firstError error) {
	event = RoamEvent{
		CurBSSID:         matches[1],
		CurFreq:          atoi(matches[2], &firstError),
		CurLevel:         atoi(matches[3], &firstError),
		CurEstThroughput: atoi(matches[4], &firstError),
		SelBSSID:         matches[5],
		SelFreq:          atoi(matches[6], &firstError),
		SelLevel:         atoi(matches[7], &firstError),
		SelEstThroughput: atoi(matches[8], &firstError),
	}

	return event, firstError
}

var eventDefs = []eventDef{
	{
		regexp.MustCompile(`CTRL-EVENT-DISCONNECTED bssid=([\da-fA-F:]+) reason=(\d+)(?: locally_generated=(1))?`),
		func(matches []string) (_ SupplicantEvent, firstError error) {
			event := new(DisconnectedEvent)
			event.BSSID = matches[1]
			event.Reason = atoi(matches[2], &firstError)
			event.LocallyGenerated = matches[3]
			event.RcvTime = time.Now()
			return event, firstError
		},
	},
	{
		regexp.MustCompile(
			`CTRL-EVENT-DO-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(matches []string) (SupplicantEvent, error) {
			event, firstError := parseRoamEvent(matches)
			event.Skip = false
			return &event, firstError
		},
	},
	{
		regexp.MustCompile(
			`CTRL-EVENT-SKIP-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(matches []string) (SupplicantEvent, error) {
			event, firstError := parseRoamEvent(matches)
			event.Skip = true
			return &event, firstError
		},
	},
	{
		regexp.MustCompile("CTRL-EVENT-SCAN-RESULTS"),
		func(matches []string) (SupplicantEvent, error) {
			return new(ScanResultsEvent), nil
		},
	},
}

// ToLogString formats the event data to string suitable for logging.
func (e *RoamEvent) ToLogString() string {
	return fmt.Sprintf("%+v\n", e)
}

// ToLogString formats the event data to string suitable for logging.
func (e *ScanResultsEvent) ToLogString() string {
	return ""
}

// ToLogString formats the event data to string suitable for logging
func (e *DisconnectedEvent) ToLogString() string {
	const timeLayout = "2006-01-02 15:04:05.000000"
	return fmt.Sprintf("%s %+v\n", e.RcvTime.Format(timeLayout), e)
}

// Start initializes the wpa_supplicant monitor.
// It starts wpa_cli process in background and creates a thread collecting its output.
// Both need to be stopped with a call to w.Stop().
func (w *WPAMonitor) Start(ctx context.Context, dutConn *ssh.Conn) error {
	cmd := dutConn.CommandContext(ctx, "sudo", "-u", "wpa", "wpa_cli")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe to wpa_cli")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe from wpa_cli")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start wpa_cli")
	}

	w.lines = make(chan string, 100)

	w.stdin = stdin
	w.stdoutScanner = bufio.NewScanner(stdout)
	w.cmd = cmd

	go func() {
		defer close(w.lines)
		for w.stdoutScanner.Scan() {
			line := w.stdoutScanner.Text()
			w.lines <- line
		}
	}()

	if err := w.waitReady(ctx); err != nil {
		w.Stop(ctx)
		return err
	}

	return nil
}

func (w *WPAMonitor) waitReady(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return errors.New("Timeout waiting for wpa_cli interactive mode start")
		case line := <-w.lines:
			if strings.Contains(line, "Interactive mode") {
				return nil
			}
		}
	}
}

// Stop sends quit command to wpa_cli and waits until the process exits (or context deadline passes).
func (w *WPAMonitor) Stop(ctx context.Context) error {
	if w.stdin == nil || w.cmd == nil {
		return errors.New("WPAMonitor not started")
	}

	if _, err := io.WriteString(w.stdin, "q\n"); err != nil {
		testing.ContextLog(ctx, "Failed to send command to wpa_cli: ", err)
	}
	w.stdin.Close()
	if err := w.cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to wait for wpa_cli exit")
	}
	// drain w.lines in case scan goroutine is stuck on writing to a full channel
	// and wait until it's closed there
	for range w.lines {
	}

	return nil
}

// ClearEvents clears all buffered output from wpa_cli, discarding events collected so far.
// Some events may still be in i/o buffers, these won't be cleared.
// Timeout (1s) is used in case of events flood.
func (w *WPAMonitor) ClearEvents(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	for len(w.lines) > 0 {
		select {
		case <-timeoutCtx.Done():
			return
		case <-w.lines:
		}
	}
}

// atoi parses string to integer, writes error to *parseErr only if it's nil
func atoi(str string, parseErr *error) int {
	value, err := strconv.Atoi(str)
	if err != nil && *parseErr == nil {
		*parseErr = err
	}
	return value
}

// WaitForEvent waits for any event in wpa_cli stdout, as defined in @eventDefs.
// It includes events already buffered in since last call to WaitForEvent or to ClearEvents.
// It returns event = nil when context deadline is exceeded.
// In case of successful match and error in parsing the fields, the event is returned (incomplete)
// and firstErr contains the first parsing error that occurred.
func (w *WPAMonitor) WaitForEvent(ctx context.Context) (event SupplicantEvent, firstErr error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case line := <-w.lines:
			if debugWPAMonitor {
				testing.ContextLog(ctx, "wpa: ", line)
			}

			for _, eventDef := range eventDefs {
				if matches := eventDef.matcher.FindStringSubmatch(line); matches != nil {
					event, error := eventDef.parseFunc(matches)
					if error != nil {
						error = errors.Wrapf(error, "error parsing line: %s", line)
					}
					return event, error
				}
			}
		}
	}
}
