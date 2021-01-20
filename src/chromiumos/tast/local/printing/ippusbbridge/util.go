// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ippusbbridge

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

// KeepAlivePath returns the path to ippusb_bridge's keepalive socket.
func KeepAlivePath(devInfo usbprinter.DevInfo) string {
	return fmt.Sprintf("/run/ippusb/%s-%s_keep_alive.sock", devInfo.VID, devInfo.PID)
}

// Kill searches the process tree to kill the ippusb_bridge process. It also
// removes the ippusb_bridge and ippusb_bridge keepalive sockets.
func Kill(ctx context.Context, devInfo usbprinter.DevInfo) error {
	ps, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return err
	}

	for _, p := range ps {
		if name, err := p.NameWithContext(ctx); err != nil || name != "ippusb_bridge" {
			continue
		}
		if status, err := p.StatusWithContext(ctx); err != nil || status == "Z" {
			// Skip child processes that have already been killed from earlier test iterations.
			continue
		}

		testing.ContextLog(ctx, "Killing ippusb_bridge with pid ", p.Pid)
		if err := syscall.Kill(int(p.Pid), syscall.SIGINT); err != nil && err != syscall.ESRCH {
			return errors.Wrap(err, "failed to kill ippusb_bridge")
		}

		// Wait for the process to exit so that its sockets can be removed.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// We need a fresh process.Process since it caches attributes.
			// TODO(crbug.com/1131511): Clean up error handling here when gpsutil has been upreved.
			if _, err := process.NewProcess(p.Pid); err == nil {
				return errors.Errorf("pid %d is still running", p.Pid)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for ippusb_bridge to exit")
		}
	}
	if err := os.Remove(fmt.Sprintf("/run/ippusb/%s-%s.sock", devInfo.VID, devInfo.PID)); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove ippusb_bridge socket")
	}
	if err := os.Remove(KeepAlivePath(devInfo)); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove ippusb_bridge keepalive socket")
	}
	return nil
}
