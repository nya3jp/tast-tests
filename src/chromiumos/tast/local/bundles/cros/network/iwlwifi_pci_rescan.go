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
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     IwlwifiPCIRescan,
		Desc:     "Makes sure that the wireless interface can recover automatically if removed",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
		// TODO(crbug/953497) add correct pci rescan dependency.
		// SoftwareDeps: []string{"iwlwifi_rescan"},
	})
}

// Reload Wifi Driver in the worst case.
func restartInterface(ctx context.Context, iface string) error {
	err := testexec.CommandContext(ctx, "modprobe", "-r", "iwlmvm", "iwlwifi").Run(testexec.DumpLogOnError)
	if err2 := testexec.CommandContext(ctx, "modprobe", "iwlwifi").Run(testexec.DumpLogOnError); err2 != nil {
		if err != nil {
			return errors.Wrap(err2, err.Error())
		}
	}
	if err2 := testexec.CommandContext(ctx, "ip", "link", "set", iface, "up").Run(testexec.DumpLogOnError); err != nil {
		if err != nil {
			return errors.Wrapf(err2, "Could not bring up %s after disable: %s", iface, err.Error())

		}
	}
	if err != nil {
		return err
	}
	return nil
}

func IwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	iface, err := network.FindWifiInterface(ctx)
	if err != nil {
		s.Fatal("Could not find valid wireless interface: ", err)
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
		newIface, err := network.FindWifiInterface(ctx)
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
