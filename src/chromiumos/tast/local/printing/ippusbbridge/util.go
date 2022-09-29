// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ippusbbridge

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

// SocketPath returns the path to ippusb_bridge's main socket.
func SocketPath(devInfo usbprinter.DevInfo) string {
	return fmt.Sprintf("/run/ippusb/%s-%s.sock", devInfo.VID, devInfo.PID)
}

// WaitForSocket waits for the ippusb_bridge socket that should match devInfo to
// appear in the filesystem.
func WaitForSocket(ctx context.Context, devInfo usbprinter.DevInfo) error {
	socket := SocketPath(devInfo)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(socket)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find ippusb_bridge socket")
	}

	return nil
}

// ContactPrinterEndpoint sends an HTTP request for url through the ippusb_bridge socket that
// matches devInfo.  Returns nil if a response is received regardless of the HTTP
// status code or body contents.
func ContactPrinterEndpoint(ctx context.Context, devInfo usbprinter.DevInfo, url string) error {
	socket := SocketPath(devInfo)

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
	}

	resp, err := client.Get("http://localhost:80" + url)
	if err != nil {
		return errors.Wrap(err, "failed to send request to ippusb_bridge socket")
	}
	_, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return errors.Wrap(err, "failed to read response from ippusb_bridge")
	}

	return nil
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
		if status, err := p.StatusWithContext(ctx); err != nil || status[0] == "Z" {
			// Skip child processes that have already been killed from earlier test iterations.
			continue
		}

		testing.ContextLog(ctx, "Killing ippusb_bridge with pid ", p.Pid)
		if err := unix.Kill(int(p.Pid), unix.SIGINT); err != nil && err != unix.ESRCH {
			return errors.Wrap(err, "failed to kill ippusb_bridge")
		}

		// Wait for the process to exit so that its sockets can be removed.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// We need a fresh process.Process since it caches attributes.
			// TODO(crbug.com/1131511): Clean up error handling here when gpsutil has been upreved.
			p, err := process.NewProcess(p.Pid)
			if err != nil {
				// Process has exited.
				return nil
			}
			status, err := p.StatusWithContext(ctx)
			if err != nil || status[0] == "Z" {
				// Process has exited but not been reaped.
				return nil
			}
			return errors.Errorf("pid %d is still running with status %s", p.Pid, status)
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for ippusb_bridge to exit")
		}
	}
	if err := os.Remove(SocketPath(devInfo)); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove ippusb_bridge socket")
	}
	return nil
}
