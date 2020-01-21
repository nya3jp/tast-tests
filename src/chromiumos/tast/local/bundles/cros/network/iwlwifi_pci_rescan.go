// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IwlwifiPCIRescan,
		Desc:         "Verifies that the WiFi interface will recover if removed when the device has iwlwifi_rescan",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"iwlwifi_rescan"},
	})
}

// restartInterface tries to reload Wifi driver when the device does not recover.
func restartInterface(ctx context.Context) error {
	err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi").Run(testexec.DumpLogOnError)
	if err != nil {
		err = errors.Wrap(err, "could not remove module iwlmvm and iwlwifi")
	}
	if err2 := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err2 != nil {
		return errors.Wrapf(err, "could not load iwlwifi module: %s", err2.Error())
	}
	return err
}

// waitIfaceRemoval waits until the interface removal from shill.
func waitIfaceRemoval(ctx context.Context, pw *shill.PropertiesWatcher, iface string) error {
	for {
		v, err := pw.WaitAll(ctx, shill.ManagerPropertyDevices)
		if err != nil {
			return err
		}
		devicePaths, ok := v[0].([]dbus.ObjectPath)
		if !ok {
			return errors.Errorf("unexpected value for devices property: %v", v)
		}
		exist := false
		for _, dPath := range devicePaths {
			dev, err := shill.NewDevice(ctx, dPath)
			if err != nil {
				return err
			}
			devProps, err := dev.GetProperties(ctx)
			if err != nil {
				if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
					// This error is forgivable as a device may go down anytime.
					continue
				}
				return err
			}
			devIface, err := devProps.GetString(shill.DevicePropertyInterface)
			if err != nil {
				// Skip the devices without valid DevicePropertyInterface
				testing.ContextLogf(ctx, "Skip device %s without valid interface, err=%s",
					dPath, err.Error())
				continue
			}
			if devIface != iface {
				continue
			}
			exist = true
			break
		}
		if !exist {
			return nil
		}
	}
}

// removeIfaceAndWait removes the interface and wait until shill does remove it.
func removeIfaceAndWait(ctx context.Context, m *shill.Manager, iface string) error {
	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return err
	}
	defer pw.Close(ctx)

	// Spawn watcher before removal.
	done := make(chan error, 1)
	watcherCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go func() {
		done <- waitIfaceRemoval(watcherCtx, pw, iface)
	}()

	driverPath := fmt.Sprintf("/sys/class/net/%s/device/remove", iface)
	if err := ioutil.WriteFile(driverPath, []byte("1"), 0200); err != nil {
		return errors.Wrapf(err, "could not remove %s driver: %s", iface, err.Error())
	}

	return <-done
}

func IwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager: ", err)
	}
	iface, err := shill.GetWifiInterface(ctx, manager, 10*time.Second)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	rescanFile := fmt.Sprintf("/sys/class/net/%s/device/driver/module/parameters/remove_when_gone", iface)
	out, err := ioutil.ReadFile(rescanFile)
	if err != nil {
		s.Fatal("Could not read rescan file: ", err)
	} else if string(out) != "Y\n" {
		s.Fatalf("wifi rescan should be enabled, current mode is %q", string(out))
	}

	testing.ContextLog(ctx, "Remove the interface and wait for shill to update")
	if err := removeIfaceAndWait(ctx, manager, iface); err != nil {
		s.Fatal("Failed to remove interface: ", err)
	}

	testing.ContextLog(ctx, "Checking the interface recovery")
	newIface, err := shill.GetWifiInterface(ctx, manager, 30*time.Second)
	if err != nil {
		restartInterface(ctx)
		s.Fatal("Device did not recover: ", err)
	} else if iface != newIface {
		restartInterface(ctx)
		s.Fatalf("looking for interface %s but got %s", iface, newIface)
	}
}
