// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/remote/wificell/router/common/support"
)

// Base contains only the base router functionality all WiFi testing router
// controllers must support.
//
// Use this in places where you are passing around a router instance where
// you do not need a specific piece of router functionality aside from what
// this supports. When you need specific supported router functionality,
// simply cast this instance to the appropriate interface or a specific router
// type directly. There are helper functions in the support package for casting
// support.Router instances to different support interfaces, which can be used
// with this as well since Base is functionality equivalent to support.Router.
type Base interface {
	support.Router
}

// Standard contains the functionality the standard WiFi testing router
// controller should support.
//
// Use this in tests if you are not specifically testing with a router that has
// non-standard support. There is no guarantee of what type of router this is; it
// just guarantees that the given router controller instance supports controlling
// these features.
//
// If you require a specific support.Type of router, use its respective router
// implementation instead.
type Standard interface {
	Base
	support.Logs
	support.Capture
	support.Hostapd
	support.DHCP
	support.IfaceManipulation
}

// StandardWithFrameSender includes all the functionality in Standard as well
// as support.FrameSender.
type StandardWithFrameSender interface {
	Standard
	support.FrameSender
}

// StandardWithBridgeAndVeth includes all the functionality in Standard as well
// as support.Bridge, support.Veth, and support.VethBridgeBinding.
type StandardWithBridgeAndVeth interface {
	Standard
	support.Bridge
	support.Veth
	support.VethBridgeBinding
}
