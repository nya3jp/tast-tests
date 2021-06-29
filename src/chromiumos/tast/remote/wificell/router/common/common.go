// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/ssh"
)

// Type is an enum indicating what type of router style a router is.
type Type int

const (
	// LegacyT is the legacy router type.
	LegacyT Type = iota
	// AxT is the ax router type.
	AxT
	// OpenWrtT is the openwrt router type.
	OpenWrtT
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

// BaseRouterStruct contains the basic router variables.
type BaseRouterStruct struct {
	// Host is the ssh connection to the router.
	Host *ssh.Conn
	// Name is the name of the Router
	Name string
	// Rtype is the router's type
	Rtype Type
}

// ReserveForRouterClose returns a shortened ctx with cancel function.
// The shortened ctx is used for running things before r.Close() to reserve time for it to run.
func ReserveForRouterClose(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.Shorten(ctx, 5*time.Second)
}
