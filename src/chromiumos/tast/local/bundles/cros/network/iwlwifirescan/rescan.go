// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iwlwifirescan provides functions used for both local/remote IwlwifiPCIRescan tests.
package iwlwifirescan

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/netiface"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

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

// waitForIfaceRemoval waits until the interface is removed from shill.
func waitForIfaceRemoval(ctx context.Context, pw *shill.PropertiesWatcher, iface string) error {
	// We use PropertiesWatcher instead of polling here. As the removal and
	// recovery might happen in the same polling cycle, in that case, we
	// will miss the interface change with polling.
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
		done <- waitForIfaceRemoval(watcherCtx, pw, iface)
	}()

	driverPath := fmt.Sprintf("/sys/class/net/%s/device/remove", iface)
	if err := ioutil.WriteFile(driverPath, []byte("1"), 0200); err != nil {
		return errors.Wrapf(err, "could not remove %s driver: %s", iface, err.Error())
	}

	return <-done
}

// RemoveIfaceAndWaitForRecovery triggers iwlwifi-rescan rule by removing the WiFi device.
// iwlwifi-rescan will rescan PCI bus and bring the WiFi device back. This function is
// used as main test body for both local and remote IwlwifiPCIRescan tests.
func RemoveIfaceAndWaitForRecovery(ctx context.Context) error {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create shill manager")
	}
	iface, err := netiface.WifiInterface(ctx, manager, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "could not get a WiFi interface")
	}
	rescanFile := fmt.Sprintf("/sys/class/net/%s/device/driver/module/parameters/remove_when_gone", iface)
	out, err := ioutil.ReadFile(rescanFile)
	if err != nil {
		return errors.Wrap(err, "could not read rescan file")
	} else if string(out) != "Y\n" {
		return errors.Errorf("wifi rescan should be enabled, current mode is %q", string(out))
	}

	// TODO(crbug.com/1048366): We now have a shill restart in pci-rescan, so we have
	// to wait until shill does restart after removing the interface in order to avoid
	// any potential restart in later code.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lock the check network hook")
	}
	defer unlock()

	// Get shill pid.
	_, _, shillPid, err := upstart.JobStatus(ctx, "shill")
	if err != nil {
		return errors.Wrap(err, "failed to get upstart status of shill")
	} else if shillPid == 0 {
		return errors.New("failed to get valid shill pid")
	}

	testing.ContextLog(ctx, "Remove the interface and wait for shill to update")
	if err := removeIfaceAndWait(ctx, manager, iface); err != nil {
		return errors.Wrap(err, "failed to remove interface")
	}

	// Wait for shill to be "running" with another pid.
	testing.ContextLog(ctx, "Wait for shill restart")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, _, pid, err := upstart.JobStatus(ctx, "shill")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "cannot get upstart status of shill"))
		}
		if pid == 0 {
			return errors.New("cannot get valid shill pid")
		}
		if pid == shillPid {
			return errors.New("shill not yet restarted")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for shill restart")
	}

	testing.ContextLog(ctx, "Checking the interface recovery")
	// Create a new manager to wait for shill being ready for dbus.
	manager, err = shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Manager object after shill restart")
	}
	newIface, err := netiface.WifiInterface(ctx, manager, 30*time.Second)
	if err != nil {
		restartInterface(ctx)
		return errors.Wrap(err, "device did not recover")
	} else if iface != newIface {
		restartInterface(ctx)
		return errors.Wrapf(err, "looking for interface %s but got %s", iface, newIface)
	}
	return nil
}
