// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UdevRename,
		Desc:     "Verifies that network interfaces remain intact after udev restart and WiFi driver rebind",
		Contacts: []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func restartWifiInterface(ctx context.Context) error {
	iface, err := shill.GetWifiInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}

	devicePath := fmt.Sprintf("/sys/class/net/%s/device", iface)
	deviceRealPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on payload %s", devicePath)
	}
	// Extract kernel name from real path.
	kernelName := filepath.Base(deviceRealPath)

	// The driver path is the directory where we can bind and release the device.
	driverPath := filepath.Join(devicePath, "driver")
	driverRealPath, err := filepath.EvalSymlinks(driverPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on path %s", driverPath)
	}

	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "unbind"), []byte(kernelName), 0200); err != nil {
		return errors.Wrapf(err, "could not unbind %s driver", iface)
	}
	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "bind"), []byte(kernelName), 0200); err != nil {
		return errors.Wrapf(err, "could not bind %s driver", iface)
	}
	return nil
}

func udevEventMonitor(ctx context.Context) <-chan error {
	done := make(chan error, 1)
	pipeRead, pipeWrite := io.Pipe()

	// Spawn udevadm monitor.
	cmd := testexec.CommandContext(ctx, "udevadm", "monitor", "-u")
	cmd.Stdout = pipeWrite
	if err := cmd.Start(); err != nil {
		done <- errors.Wrap(err, "failed to spawn \"udevadm monitor\"")
		return done
	}

	// Spawn watch routine (and let the routine to cleanup related resource).
	go func() {
		defer pipeRead.Close()
		defer pipeWrite.Close()
		defer func() {
			if err := cmd.Kill(); err != nil {
				testing.ContextLog(ctx, "Failed to kill udevadm monitor")
			}
		}()

		readErr := make(chan error, 1)
		// Spawn the reader, as io.Pipe is synchronous, we need another routine
		// so that we can also watch ctx.Done.
		go func() {
			buf := make([]byte, 1)
			count, err := pipeRead.Read(buf)
			if count == 0 || err != nil {
				readErr <- errors.New("udev event not captured")
			} else {
				readErr <- nil
			}
		}()

		// Wait response from child or cleanup when time's up.
		select {
		case err := <-readErr:
			done <- err
		case <-ctx.Done():
			done <- errors.New("did not receive udev response before timeout")
		}
	}()
	return done
}

func restartUdev(ctx context.Context) error {
	const service = "udev"
	if _, state, _, err := upstart.JobStatus(ctx, service); err != nil {
		return errors.Wrapf(err, "could not query status of service %s", service)
	} else if state != upstart.RunningState {
		return errors.Errorf("%s not running", service)
	}

	if err := upstart.StopJob(ctx, service); err != nil {
		return errors.Errorf("%s failed to stop", service)
	}

	// Make sure udev finished its job and stopped.
	testexec.CommandContext(ctx, "udevadm", "settle").Run()

	// TODO(yenlinlai): Currently we don't yet have a good way to wait from restarting
	// udev until having all rules processed. "udevadm settle" may not properly wait if
	// udev has not gotten into event processing loop. Some examples can be found in
	// crrev.com/c/1725184.
	// Our current work-around is to watch the first output of "udevadm monitor -u" as
	// the ready signal. However, there's still some possible race if udev finishes all
	// update before udevadm starts.

	// Spawn udevadm monitor, continue when error cause we want to start udev.
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := udevEventMonitor(timeoutCtx)

	if err := upstart.StartJob(ctx, service); err != nil {
		return errors.Errorf("%s failed to start", service)
	}

	return <-done
}

// deviceRestarter is a function type that defines a first class function that would restart
// a device or series of devices. restartUdev() and restartWifiInterface() match the
// function prototype.
type deviceRestarter func(ctx context.Context) error

func testUdevDeviceList(ctx context.Context, fn deviceRestarter) error {
	iflistPre, err := network.Interfaces()
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}

	// Wait for event processing.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Second))
	defer cancel()
	if err := testexec.CommandContext(timeoutCtx, "udevadm", "settle").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "device could not settle in time after restart")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		iflistPost, err := network.Interfaces()
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(iflistPre, iflistPost) {
			return errors.Errorf("unexpected network interfaces: got %v, want %v", iflistPost, iflistPre)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}

func UdevRename(ctx context.Context, s *testing.State) {
	if err := testUdevDeviceList(ctx, restartUdev); err != nil {
		s.Error("Restarting udev: ", err)
	}

	if err := testUdevDeviceList(ctx, restartWifiInterface); err != nil {
		s.Error("Restarting wireless interface: ", err)
	}
}
