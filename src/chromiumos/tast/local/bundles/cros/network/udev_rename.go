// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UdevRename,
		Desc:     "Verifies that network interfaces don't disappear completely after udev or the interface drivers are restarted",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func restartWirelessInterface(ctx context.Context) error {
	iface, err := network.FindWirelessInterface(ctx)
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}

	devicePath := fmt.Sprintf("/sys/class/net/%s/device", iface)
	deviceRealPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on payload %s", devicePath)
	}
	// The payload being written to the file is the device name gleaned
	// from the baseDir of the deviceRealPath.
	payload := filepath.Base(deviceRealPath)

	// The driver path is the directory where we can bind and release the device.
	driverPath := filepath.Join(devicePath, "driver")
	driverRealPath, err := filepath.EvalSymlinks(driverPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on path %s", driverPath)
	}

	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "unbind"), []byte(payload), 0200); err != nil {
		return errors.Wrapf(err, "could not unbind %s driver", iface)
	}
	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "bind"), []byte(payload), 0200); err != nil {
		return errors.Wrapf(err, "could not bind %s driver", iface)
	}
	return nil
}

func restartUdev(ctx context.Context) error {
	const service = "udev"
	if _, state, _, err := upstart.JobStatus(ctx, service); err != nil {
		return errors.Wrapf(err, "could not query status of service %s", service)
	} else if state != upstart.RunningState {
		return errors.Errorf("%s not running", service)
	}

	if err := upstart.RestartJob(ctx, service); err != nil {
		return errors.Errorf("%s failed to restart", service)
	}
	return nil
}

// deviceRestarter is a function prototype that defines a first class function that would restart
// a device or series of devices. restartUdev and restartWirelessInterface match the
// function prototype.
type deviceRestarter func(ctx context.Context) error

func testUdevDeviceList(ctx context.Context, fn deviceRestarter) error {
	iflistPre, err := network.GetInterfaceList()
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}

	// We need to wait for udev to rename (or not) the interface!
	// This is to prevent false negatives if we poll the interface list before the
	// restart actually occurs..
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Second))
	defer cancel()
	if err := testexec.CommandContext(timeoutCtx, "udevadm", "settle").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "device could not settle intime after device restart")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		iflistPost, err := network.GetInterfaceList()
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

	if err := testUdevDeviceList(ctx, restartWirelessInterface); err != nil {
		s.Error("Restarting wireless interface: ", err)
	}
}
