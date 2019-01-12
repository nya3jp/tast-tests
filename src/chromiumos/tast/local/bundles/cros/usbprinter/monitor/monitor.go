// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package monitor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

func mySplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := strings.Index(string(data), "\n\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return
}

// PrinterEvent monitors USB events using udevadm and waits to see if a USB
// event with |action| occurs for a device with vendor ID |vid| and product ID
// |pid|. The |ret| channel is used to communicate results to the caller.
func PrinterEvent(ctx context.Context, ret chan error, action, vid,
	pid string) {
	cmd := testexec.CommandContext(ctx, "stdbuf", "-o0", "udevadm", "monitor",
		"--subsystem-match=usb", "--property", "--udev")

	p, err := cmd.StdoutPipe()
	if err != nil {
		ret <- err
	}

	if err := cmd.Start(); err != nil {
		ret <- err
	}
	defer cmd.Wait()
	defer cmd.Kill()

	ch := make(chan error, 1)

	// Scan through the output from udevadm and look for a line which matches the
	// expected |vid|, |pid|, and |action|.
	go func() {
		sc := bufio.NewScanner(p)
		sc.Split(mySplit)
		for sc.Scan() {
			output := sc.Text()
			matchVid := fmt.Sprintf("ID_VENDOR_ID=%s", vid)
			matchPid := fmt.Sprintf("ID_MODEL_ID=%s", pid)
			matchAction := fmt.Sprintf("ACTION=%s", action)

			if strings.Contains(output, matchVid) &&
				strings.Contains(output, matchPid) &&
				strings.Contains(output, matchAction) {
				ch <- nil
				return
			}
		}

		if sc.Err() != nil {
			ch <- sc.Err()
		} else {
			ch <- errors.Errorf("Failed to find matching udev event: %s %s %s", action,
				vid, pid)
		}
	}()

	select {
	case err := <-ch:
		ret <- err
	case <-time.After(2 * time.Second):
		ret <- errors.New("Timed out before receiving event")
	case <-ctx.Done():
		ret <- errors.Errorf("Failed to get result: %s", err)
	}
}

// PrinterLaunch scans the output from virtual-usb-printer using the pipe |p|
// for the prompt which indicates that the virtual printer has started and is
// ready to begin handling requests.
func PrinterLaunch(ctx context.Context, p io.ReadCloser) error {
	ch := make(chan error, 1)

	go func() {
		sc := bufio.NewScanner(p)
		for sc.Scan() {
			if sc.Text() == "virtual-usb-printer: ready to accept connections" {
				ch <- nil
			}
		}

		if sc.Err() != nil {
			ch <- sc.Err()
		} else {
			ch <- errors.New("Failed to find prompt for printer launch")
		}
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		return errors.New("Timed out before receiving event")
	case <-ctx.Done():
		return errors.New("Failed to start printer")
	}
}
