// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbprinter provides an interface to configure and attach a virtual
// USB printer onto the system to be used for testing.
package usbprinter

import (
	"bufio"
	"context"
	"io"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// waitEvent monitors USB events using udevadm and waits to see if a USB event
// with |action| occurs for a device with vendor ID |vid| and product ID |pid|.
// Returns nil if a matching event is found.
func waitEvent(ctx context.Context, action, vid, pid string) error {
	cmd := testexec.CommandContext(ctx, "stdbuf", "-o0", "udevadm", "monitor",
		"--subsystem-match=usb", "--property", "--udev")

	p, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	rd := bufio.NewReader(p)
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	defer cmd.Kill()

	ch := make(chan error, 1)

	// Scan through the output from udevadm and look for a reported event which
	// matches the expected |vid|, |pid|, and |action|.
	go func() {
		matchVID := "ID_VENDOR_ID=" + vid
		matchPID := "ID_MODEL_ID=" + pid
		matchAction := "ACTION=" + action
		var sb strings.Builder
		for {
			if ctx.Err() != nil {
				ch <- ctx.Err()
				return
			}
			text, err := rd.ReadString('\n')
			if err != nil {
				ch <- errors.Wrap(err, "failed to read output from pipe")
				return
			}
			if text != "\n" {
				sb.WriteString(text)
				continue
			}
			// If we read an empty line, it means that we have reached the end of
			// the chunk. Check the contents of the chunk to see if it matches the
			// expected event.
			chunk := sb.String()
			if strings.Contains(chunk, matchVID) &&
				strings.Contains(chunk, matchPID) &&
				strings.Contains(chunk, matchAction) {
				ch <- nil
				return
			}
			// Reset if there was no match found on the previous chunk.
			sb.Reset()
		}
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get udev event")
	}
}

// waitLaunch scans the output from virtual-usb-printer using the pipe |p| for
// the prompt which indicates that the virtual printer has started and is
// ready to begin handling requests.
func waitLaunch(ctx context.Context, r io.Reader) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "printer didn't report readiness")
		}
		if sc.Text() == "virtual-usb-printer: ready to accept connections" {
			return nil
		}
	}
	if sc.Err() != nil {
		return errors.Wrap(sc.Err(), "failed to read from scanner")
	}
	return errors.New("failed to find prompt for printer launch")
}
