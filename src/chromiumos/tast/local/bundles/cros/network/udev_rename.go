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
	"strings"
	"time"

	"chromiumos/tast/errors"
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

func findWirelessInterface() (string, error) {
	typeList := []string{"wlan", "mlan"}
	ifaceList, err := getInterfaceList()
	if err != nil {
		return "", err
	}
	for _, pref := range typeList {
		for _, intf := range ifaceList {
			if strings.HasPrefix(intf, pref) {
				return intf, nil
			}
		}
	}
	return "", errors.New("could not find an interface")
}

func getInterfaceList() ([]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	toRet := make([]string, len(files), len(files))
	for i, file := range files {
		toRet[i] = file.Name()
	}
	return toRet, nil
}

func restartWirelessInterface(ctx context.Context) error {
	iface, err := findWirelessInterface()
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}

	devicePath := fmt.Sprintf("/sys/class/net/%s/device", iface)
	deviceRealPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on payload %s", devicePath)
	}
	// The payload being written to the file is the device name gleaned
	// from the baseDir of the deviceRealPath
	payload := filepath.Base(deviceRealPath)

	// The driver path is the directory where we can bind and release the device.
	driverPath := filepath.Join(devicePath, "driver")
	driverRealPath, err := filepath.EvalSymlinks(driverPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on path %s", driverPath)
	}

	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "unbind"), []byte(payload), 0200); err != nil {
		return errors.Wrapf(err, "could not unbind driver, %s", iface)
	}
	if err := ioutil.WriteFile(filepath.Join(driverRealPath, "bind"), []byte(payload), 0200); err != nil {
		return errors.Wrapf(err, "could not bind driver, %s", iface)
	}
	return nil
}

func restartUdev(ctx context.Context) error {
	const service = "udev"

	if !upstart.JobExists(ctx, service) {
		return errors.Errorf("%s not running", service)
	}
	if err := upstart.RestartJob(ctx, service); err != nil {
		return errors.Errorf("%s failed to restart", service)
	}
	if !upstart.JobExists(ctx, service) {
		return errors.Errorf("%s failed to stay running", service)
	}
	return nil
}

// deviceRestarter is a function prototype that defines a first class function that would restart
// a device or series of devices. restartUdev and restartWirelessInterface match the
// function prototype.
type deviceRestarter func(ctx context.Context) error

func testUdevDeviceList(ctx context.Context, fn deviceRestarter) error {
	iflistPre, err := getInterfaceList()
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		iflistPost, err := getInterfaceList()
		if err != nil {
			return err
		}
		if reflect.DeepEqual(iflistPre, iflistPost) {
			return nil
		}
		return errors.Errorf("Interface changed %v != %v", iflistPre, iflistPost)
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}

func UdevRename(ctx context.Context, s *testing.State) {
	if err := testUdevDeviceList(ctx, restartUdev); err != nil {
		s.Error(errors.Wrap(err, "restartUdev failed"))
	}

	if err := testUdevDeviceList(ctx, restartWirelessInterface); err != nil {
		s.Error(errors.Wrap(err, "restartWirelessInterface failed"))
	}
}
