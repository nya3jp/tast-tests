// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"bufio"
	"context"
	"io"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// waitEvent monitors USB events using udevadm and waits to see if a USB event
// with action occurs for a device with vendor ID vid and product ID pid.
// Writes the process id of the child process to onPidReady once udevadm is ready for
// events, or closes onPidReady without writing anything if an error occurs.
// Returns nil if a matching event is found.
func waitEvent(ctx context.Context, action string, devInfo DevInfo, onPidReady chan<- int) error {
	cmd := testexec.CommandContext(ctx, "stdbuf", "-o0", "udevadm", "monitor",
		"--subsystem-match=usb", "--property", "--udev")

	p, err := cmd.StdoutPipe()
	if err != nil {
		close(onPidReady)
		return err
	}
	if err := cmd.Start(); err != nil {
		close(onPidReady)
		return err
	}

	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	// Scan through the output from udevadm and look for a reported event which
	// matches the expected devInfo and action.
	matchVID := "ID_VENDOR_ID=" + devInfo.VID
	matchPID := "ID_MODEL_ID=" + devInfo.PID
	matchAction := "ACTION=" + action
	var sb strings.Builder
	rd := bufio.NewReader(p)
	notified := false
	for {
		if ctx.Err() != nil {
			close(onPidReady)
			return errors.Wrap(ctx.Err(), "didn't get udev event")
		}
		text, err := rd.ReadString('\n')
		if err != nil {
			close(onPidReady)
			return errors.Wrap(err, "failed to read output from pipe")
		}
		if text != "\n" {
			sb.WriteString(text)
			continue
		}
		// Once we read the first blank line of output from udevadm, pass its PID
		// along to the caller to tell them udevadm is ready to receive events.
		if !notified {
			onPidReady <- cmd.Cmd.Process.Pid
			close(onPidReady)
			notified = true
		}
		// If we read an empty line, it means that we have reached the end of
		// the chunk. Check the contents of the chunk to see if it matches the
		// expected event.
		chunk := sb.String()
		if strings.Contains(chunk, matchVID) &&
			strings.Contains(chunk, matchPID) &&
			strings.Contains(chunk, matchAction) {
			return nil
		}
		// Reset if there was no match found on the previous chunk.
		sb.Reset()
	}
}

// startUdevMonitor launches udevadm in a goroutine and waits until the child
// process emits headers indicating it is ready to receive events.  The returned
// channel can be read from to wait for an event that matches action and devInfo.
func startUdevMonitor(ctx context.Context, action string, devInfo DevInfo) (<-chan error, error) {
	udevPid := make(chan int, 1)
	udevCh := make(chan error, 1)
	go func() {
		udevCh <- waitEvent(ctx, action, devInfo, udevPid)
	}()
	pid, ok := <-udevPid
	if !ok {
		return nil, errors.Wrap(ctx.Err(), "didn't get udevadm PID")
	}
	testing.ContextLogf(ctx, "udevadm with PID %d is ready", pid)
	return udevCh, nil
}

// waitLaunch scans r, which contains output from virtual-usb-printer, for the
// prompt indicating that it has launched successfully.
func waitLaunch(r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if sc.Text() == "virtual-usb-printer: ready to accept connections" {
			return nil
		}
	}
	if sc.Err() != nil {
		return errors.Wrap(sc.Err(), "failed to read from scanner")
	}
	return errors.New("failed to find prompt for printer launch")
}
