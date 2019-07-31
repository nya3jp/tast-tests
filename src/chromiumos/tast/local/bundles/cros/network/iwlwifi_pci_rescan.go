// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
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

// restartInterface tries to reload Wifi driver when the device do not recover.
func restartInterface(ctx context.Context, iface string) error {
	err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi").Run(testexec.DumpLogOnError)
	if err != nil {
		err = errors.Wrap(err, "could not remove module iwlmvm and iwlwifi")
	}
	if err2 := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err2 != nil {
		return errors.Wrapf(err, "could not load iwlwifi module: %s", err2.Error())
	}
	if err2 := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err2 != nil {
		return errors.Wrapf(err, "could not bring up %s after disable: %s", iface, err2.Error())
	}
	return err
}

func IwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	iface, err := shill.GetWifiInterface(ctx)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	rescanFile := fmt.Sprintf("/sys/class/net/%s/device/driver/module/parameters/remove_when_gone", iface)
	out, err := ioutil.ReadFile(rescanFile)
	if err != nil {
		s.Fatal("Could not read rescan file: ", err)
	} else if string(out) != "Y\n" {
		s.Fatalf("wifi rescan should be enabled, current mode is %s", string(out))
	}
	driverPath := fmt.Sprintf("/sys/class/net/%s/device/remove", iface)
	driverRealPath, err := filepath.EvalSymlinks(driverPath)

	if err := ioutil.WriteFile(driverRealPath, []byte("1"), 0200); err != nil {
		s.Fatalf("Could not remove %s driver: %s", iface, err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newIface, err := shill.GetWifiInterface(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to find wireless interface")
		}
		if iface == newIface {
			return nil
		}
		return errors.Errorf("looking for interface %s but got %s", iface, newIface)

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		defer func() {
			err := restartInterface(ctx, iface)
			if err != nil {
				s.Error("Failed to restart interface: ", err)
			}
		}()
		s.Fatal("Device did not recover: ", err)
	}
}
