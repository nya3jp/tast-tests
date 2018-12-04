// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
// with action occurs for a device with vendor ID vid and product ID pid.
// Returns nil if a matching event is found.
func waitEvent(ctx context.Context, action, vid, pid string) error {
	cmd := testexec.CommandContext(ctx, "stdbuf", "-o0", "udevadm", "monitor",
		"--subsystem-match=usb", "--property", "--udev")

	p, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	// Scan through the output from udevadm and look for a reported event which
	// matches the expected |vid|, |pid|, and |action|.
	matchVID := "ID_VENDOR_ID=" + vid
	matchPID := "ID_MODEL_ID=" + pid
	matchAction := "ACTION=" + action
	var sb strings.Builder
	rd := bufio.NewReader(p)
	for {
		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "didn't get udev event")
		}
		text, err := rd.ReadString('\n')
		if err != nil {
			return errors.Wrap(err, "failed to read output from pipe")
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
			return nil
		}
		// Reset if there was no match found on the previous chunk.
		sb.Reset()
	}
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
