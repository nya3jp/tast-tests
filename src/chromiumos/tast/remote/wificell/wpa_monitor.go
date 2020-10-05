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

const debug = false

// Event defines functions common for all wpa_supplicant events
type Event interface {
	ToLogString() string
}

// RoamEvent defines data of CTRL-EVENT-DO-ROAM and CTRL-EVENT-SKIP-ROAM events
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

// ScanResultsEvent defines data of CTRL-EVENT-SCAN-RESULTS event
type ScanResultsEvent struct {
}

// WPAMonitor holds internal context of the WPA monitor
type WPAMonitor struct {
	stdin         io.WriteCloser
	stdoutScanner *bufio.Scanner
	cmd           *ssh.Cmd
	lines         chan string
	done          chan bool
}

type eventDef struct {
	matcher   *regexp.Regexp
	parseFunc func(ctx context.Context, matches []string) (event Event)
}

var eventDefs = []eventDef{
	{
		regexp.MustCompile(
			`CTRL-EVENT-DO-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(ctx context.Context, matches []string) (parsedEvent Event) {
			event := new(RoamEvent)
			event.CurBSSID = matches[1]
			event.CurFreq = atoiLog(ctx, matches[2])
			event.CurLevel = atoiLog(ctx, matches[3])
			event.CurEstThroughput = atoiLog(ctx, matches[4])
			event.SelBSSID = matches[5]
			event.SelFreq = atoiLog(ctx, matches[6])
			event.SelLevel = atoiLog(ctx, matches[7])
			event.SelEstThroughput = atoiLog(ctx, matches[8])
			event.Skip = false
			return event
		},
	},
	{
		regexp.MustCompile(
			`CTRL-EVENT-SKIP-ROAM cur_bssid=([\da-fA-F:]+) cur_freq=(\d+) ` +
				`cur_level=([\d-]+) cur_est=(\d+) ` +
				`sel_bssid=([\da-fA-F:]+) sel_freq=(\d+) ` +
				`sel_level=([\d-]+) sel_est=(\d+)`),
		func(ctx context.Context, matches []string) (parsedEvent Event) {
			event := new(RoamEvent)
			event.CurBSSID = matches[1]
			event.CurFreq = atoiLog(ctx, matches[2])
			event.CurLevel = atoiLog(ctx, matches[3])
			event.CurEstThroughput = atoiLog(ctx, matches[4])
			event.SelBSSID = matches[5]
			event.SelFreq = atoiLog(ctx, matches[6])
			event.SelLevel = atoiLog(ctx, matches[7])
			event.SelEstThroughput = atoiLog(ctx, matches[8])
			event.Skip = true
			return event
		},
	},
	{
		regexp.MustCompile("CTRL-EVENT-SCAN-RESULTS"),
		func(ctx context.Context, matches []string) (parsedEvent Event) {
			return new(ScanResultsEvent)
		},
	},
}

// ToLogString formats the event data to string suitable for logging
func (e *RoamEvent) ToLogString() string {
	return fmt.Sprintf("%+v\n", e)
}

// ToLogString formats the event data to string suitable for logging
func (e *ScanResultsEvent) ToLogString() string {
	return ""
}

// Start initializes the wpa_supplicant monitor.
// It starts wpa_cli process in background and creates a thread collecting its output.
// Both need to be stopped with a call to w.Stop()
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
	w.done = make(chan bool)

	go func() {
		for w.stdoutScanner.Scan() {
			line := w.stdoutScanner.Text()
			w.lines <- line
		}
		w.done <- true
	}()

	return nil
}

// Stop sends quit command to wpa_cli and waits until the process exits (or context deadline passes)
func (w *WPAMonitor) Stop(ctx context.Context) (err error) {
	_, err = io.WriteString(w.stdin, "q\n")
	if err != nil {
		return errors.Wrap(err, "failed to send command to wpa_cli")
	}

	select {
	case <-ctx.Done():
		return context.DeadlineExceeded
	case <-w.done:
		return nil
	}
}

// ClearEvents clears all buffered output from wpa_cli
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

// WaitForEvent waits for any event in wpa_cli stdout, as defined in @eventDefs
// It returns (nil, nil) when context deadline is exceeded.
func (w *WPAMonitor) WaitForEvent(ctx context.Context) (event Event, err error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case line := <-w.lines:
			if debug {
				testing.ContextLog(ctx, "wpa: ", line)
			}

			for _, eventDef := range eventDefs {
				matches := eventDef.matcher.FindStringSubmatch(line)
				if matches != nil {
					event := eventDef.parseFunc(ctx, matches)
					return event, nil
				}
			}
		}
	}

	return nil, io.EOF
}
