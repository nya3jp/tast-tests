// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/stringset"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UdevRename,
		Desc: "Verifies that network interfaces remain intact after udev restart and WiFi driver rebind",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr: []string{"group:mainline"},
		// TODO(b/149247291): remove the elm/hana 3.18 dependency once elm/hana upreved kernel to 4.19 or above.
		SoftwareDeps: []string{"wifi", "shill-wifi", "no_elm_hana_3_18"},
		// TODO(b/183992356): remove informational variant once Asurada unbind/bind issues are fixed.
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("asurada")),
			},
			{
				Name:              "informational",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("asurada")),
			},
		},
	})
}

func restartWifiInterface(ctx context.Context) error {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating shill manager proxy")
	}

	iface, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return errors.Wrap(err, "could not find interface")
	}

	devicePath := fmt.Sprintf("/sys/class/net/%s/device", iface)
	deviceRealPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on payload %s", devicePath)
	}

	// The driver path is the directory where we can bind and release the device.
	driverPath := filepath.Join(devicePath, "driver")
	driverRealPath, err := filepath.EvalSymlinks(driverPath)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate symlink on path %s", driverPath)
	}

	// Function to find device paths for the brcmfmac (Broadcom FullMAC) driver.
	// In general, one device is associated with a driver. However, for brcmfmac driver,
	// it associates with two devices. We have to unbind/bind both.
	brcmfmacDevicePaths := func(driverPath string) ([]string, error) {
		paths, err := filepath.Glob(filepath.Join(driverPath, "*"))
		if err != nil {
			return nil, err
		}
		if len(paths) <= 1 {
			return nil, errors.Errorf("found %d brcmfmac driver devices, expected at least 2", len(paths))
		}

		var ret []string
		for _, p := range paths {
			// Only consider links to devices, and not paths like '/sys/bus/.../unbind'.
			if rp, err := filepath.EvalSymlinks(p); err == nil && strings.HasPrefix(rp, "/sys/devices") {
				ret = append(ret, rp)
			}
		}
		return ret, nil
	}

	devPaths := []string{deviceRealPath}
	// Special case for brcmfmac (Broadcom FullMAC) driver.
	// Note that in older kernels, e.g. 3.14, the driver name of Broadcom FullMAC is "brcmfmac_sdio";
	// however, in recent kernels, e.g. 4.19, it is named as "brcmfmac". So we use prefix match here.
	if strings.HasPrefix(filepath.Base(driverRealPath), "brcmfmac") {
		devPaths, err = brcmfmacDevicePaths(driverPath)
		if err != nil {
			errors.Wrap(err, "brcmfmac device paths error")
		}
		testing.ContextLog(ctx, "Devices associated with brcmfmac driver: ", devPaths)
	}

	for _, devPath := range devPaths {
		testing.ContextLogf(ctx, "Rebind device %s to driver %s", devPath, driverRealPath)
		devName := filepath.Base(devPath)
		if err := ioutil.WriteFile(filepath.Join(driverRealPath, "unbind"), []byte(devName), 0200); err != nil {
			return errors.Wrapf(err, "could not unbind %s driver", iface)
		}
		if err := ioutil.WriteFile(filepath.Join(driverRealPath, "bind"), []byte(devName), 0200); err != nil {
			return errors.Wrapf(err, "could not bind %s driver", iface)
		}
	}
	return nil
}

func restartUdev(ctx context.Context) error {
	const service = "udev"
	if _, state, _, err := upstart.JobStatus(ctx, service); err != nil {
		return errors.Wrapf(err, "could not query status of service %s", service)
	} else if state != upstartcommon.RunningState {
		return errors.Errorf("%s not running", service)
	}

	if err := upstart.RestartJob(ctx, service); err != nil {
		return errors.Errorf("%s failed to restart", service)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "udevadm", "control", "--ping").Run(); err != nil {
			return errors.Wrap(err, "udev is not yet running")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}

// deviceRestarter is a function type that defines a first class function that would restart
// a device or series of devices. restartUdev() and restartWifiInterface() match the
// function prototype.
type deviceRestarter func(ctx context.Context) error

func interfaceNames() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	// Sanitize iface names to exclude ARC and Crostini related interfaces.
	// Context: b/191789332
	var names []string
	for _, iface := range ifaces {
		name := iface.Name
		if !(strings.HasPrefix(name, "arc") || strings.HasPrefix(name, "vmtap")) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// expectIface expects actual interfaces is the same as expected.
func expectIface(expect, actual []string) error {
	es := stringset.New(expect)
	as := stringset.New(actual)
	if es.Equal(as) {
		return nil
	}
	var errs []string

	// wanted: interfaces in expect not in actual.
	if wanted := es.Diff(as); len(wanted) > 0 {
		errs = append(errs, fmt.Sprintf("wanted:%v", wanted.Elements()))
	}
	// unexpected: interfaces in actual not in expect.
	if unexpected := as.Diff(es); len(unexpected) > 0 {
		errs = append(errs, fmt.Sprintf("unexpected:%v", unexpected.Elements()))
	}
	// matched: interfaces in both actual and expect.
	if matched := es.Intersect(as); len(matched) > 0 {
		errs = append(errs, fmt.Sprintf("matched:%v", matched.Elements()))
	}
	return errors.New("failed expecting network interfaces: " + strings.Join(errs, ", "))
}

func testUdevDeviceList(ctx context.Context, fn deviceRestarter) error {
	iflistPre, err := interfaceNames()
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}

	// Wait for event processing.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(5*time.Second))
	defer cancel()
	if err := testexec.CommandContext(timeoutCtx, "udevadm", "settle").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "device could not settle in time after restart")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		iflistPost, err := interfaceNames()
		if err != nil {
			return err
		}
		if err := expectIface(iflistPre, iflistPost); err != nil {
			return err
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
