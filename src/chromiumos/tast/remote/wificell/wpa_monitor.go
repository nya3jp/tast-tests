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

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const debugWPAMonitor = false

// SupplicantEvent defines functions common for all wpa_supplicant events
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
	parseFunc func(ctx context.Context, matches []string) (event SupplicantEvent)
}

func parseRoamEvent(ctx context.Context, matches []string) (event RoamEvent) {
	return RoamEvent{
		CurBSSID:         matches[1],
		CurFreq:          atoiLog(ctx, matches[2]),
		CurLevel:         atoiLog(ctx, matches[3]),
		CurEstThroughput: atoiLog(ctx, matches[4]),
		SelBSSID:         matches[5],
		SelFreq:          atoiLog(ctx, matches[6]),
		SelLevel:         atoiLog(ctx, matches[7]),
		SelEstThroughput: atoiLog(ctx, matches[8]),
	}
}

var eventDefs = []eventDef{
	{
		regexp.MustCompile(
			`CTRL-EVENT-DO-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(ctx context.Context, matches []string) (parsedEvent SupplicantEvent) {
			event := parseRoamEvent(ctx, matches)
			event.Skip = false
			return &event
		},
	},
	{
		regexp.MustCompile(
			`CTRL-EVENT-SKIP-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(ctx context.Context, matches []string) (parsedEvent SupplicantEvent) {
			event := parseRoamEvent(ctx, matches)
			event.Skip = true
			return &event
		},
	},
	{
		regexp.MustCompile("CTRL-EVENT-SCAN-RESULTS"),
		func(ctx context.Context, matches []string) (parsedEvent SupplicantEvent) {
			return new(ScanResultsEvent)
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

// Start initializes the wpa_supplicant monitor.
// It starts wpa_cli process in background and creates a thread collecting its output.
// Both need to be stopped with a call to w.Stop().
func (w *WPAMonitor) Start(ctx context.Context, dutConn *ssh.Conn) (err error) {
	cmd := dutConn.Command("sudo", "-u", "wpa", "wpa_cli")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe to wpa_cli")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe from wpa_cli")
	}

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start wpa_cli")
	}

	w.lines = make(chan string, 100)

	w.stdin = stdin
	w.stdoutScanner = bufio.NewScanner(stdout)
	w.cmd = cmd

	go func() {
		defer func() {
			// panic can happen if w.lines is closed while blocked on write here
			if panic := recover(); panic != nil {
				testing.ContextLog(ctx, "Panic in wpa_monitor scan thread: ", panic)
			}
		}()
		for w.stdoutScanner.Scan() {
			line := w.stdoutScanner.Text()
			w.lines <- line
		}
	}()

	return nil
}

// Stop sends quit command to wpa_cli and waits until the process exits (or context deadline passes).
func (w *WPAMonitor) Stop(ctx context.Context) (err error) {
	if _, err = io.WriteString(w.stdin, "q\n"); err != nil {
		return errors.Wrap(err, "failed to send command to wpa_cli")
	}
	w.stdin.Close()
	if err = w.cmd.Wait(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for wpa_cli exit")
	}
	// drain w.lines in case scan goroutine is stuck on writing to a full channel
	w.ClearEvents()
	close(w.lines)

	return nil
}

// ClearEvents clears all buffered output from wpa_cli, discarding events collected so far.
func (w *WPAMonitor) ClearEvents() {
	for len(w.lines) > 0 {
		<-w.lines
	}
}

func atoiLog(ctx context.Context, str string) int {
	val, err := strconv.Atoi(str)
	if err != nil {
		testing.ContextLog(ctx, "Failed to convert to int:", str)
	}
	return val
}

// WaitForEvent waits for any event in wpa_cli stdout, as defined in @eventDefs.
// It returns nil when context deadline is exceeded.
func (w *WPAMonitor) WaitForEvent(ctx context.Context) (event SupplicantEvent) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case line := <-w.lines:
			if debugWPAMonitor {
				testing.ContextLog(ctx, "wpa: ", line)
			}

			for _, eventDef := range eventDefs {
				if matches := eventDef.matcher.FindStringSubmatch(line); matches != nil {
					return eventDef.parseFunc(ctx, matches)
				}
			}
		}
	}
}
