// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	"chromiumos/tast/common/network/ip"
)

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.

	// AutotestWorkdirGlob is the path that grabs all autotest outputs.
	AutotestWorkdirGlob = "/tmp/autotest-*"
	// WorkingDir is the tast-test's working directory.
	WorkingDir = "/tmp/tast-test/"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc

	// VethPrefix is the prefix for the veth interface.
	VethPrefix = "vethA"
	// VethPeerPrefix is the prefix for the peer's veth interface.
	VethPeerPrefix = "vethB"
	// BridgePrefix is the prefix for the bridge interface.
	BridgePrefix = "tastbr"
)

// RouterCloseContextDuration is a shorter context.Context duration is used for running things before Router.Close to reserve time for it to run.
const RouterCloseContextDuration = 5 * time.Second

// RemoveDevicesWithPrefix removes the devices whose names start with the given prefix.
func RemoveDevicesWithPrefix(ctx context.Context, ipr *ip.Runner, prefix string) error {
	devs, err := ipr.LinkWithPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if err := ipr.DeleteLink(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}
