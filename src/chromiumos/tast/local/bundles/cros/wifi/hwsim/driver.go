// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsim

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const simulatedWiFiDriver = "mac80211_hwsim"

// load loads the driver into the host kernel and asks for "n" simulated interfaces.
// Returns the list of network interfaces that belong to the driver.
func load(ctx context.Context, n int) ([]net.Interface, error) {
	var ifaces []net.Interface

	// Load the driver
	cmd := testexec.CommandContext(ctx, "modprobe", simulatedWiFiDriver, fmt.Sprintf("radios=%d", n))
	if err := cmd.Run(); err != nil {
		return ifaces, errors.Wrap(err, "failed to load mac80211_hwsim")
	}

	// List the interfaces that belong to the driver.
	netIfaces, err := net.Interfaces()
	if err != nil {
		return ifaces, errors.Wrap(err, "failed to list network interfaces")
	}
	for _, iface := range netIfaces {
		if belongsToDriver(iface.Name) {
			ifaces = append(ifaces, iface)
		}
	}

	return ifaces, nil
}

// unload unloads the driver from the device.
func unload(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-r", simulatedWiFiDriver)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to unload mac80211_hwsim")
	}
	return nil
}

// isLoaded returns true when mac80211_hwsim is loaded.
func isLoaded() (bool, error) {
	path := filepath.Join("/sys/module", simulatedWiFiDriver)
	var err error
	if fi, err := os.Stat(path); err == nil {
		return fi.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// belongsToDriver returns true when iface belongs to mac80211_hwsim driver.
func belongsToDriver(iface string) bool {
	modulePath := filepath.Join("/sys/class/net", iface, "device/driver/module")
	module, err := os.Readlink(modulePath)
	if err != nil {
		return false
	}
	return strings.Contains(module, simulatedWiFiDriver)
}
