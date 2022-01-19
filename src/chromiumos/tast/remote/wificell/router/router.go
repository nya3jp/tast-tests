// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/remote/wificell/router/common/support"
)

// Standard contains the functionality the standard WiFi testing router controller should support.
//
// Use this in tests if you are not specifically testing with a router that has
// non-standard support. There is no guarantee of what type of router this is; it
// just guarantees that the given router controller instance supports controlling
// these features.
//
// If you require a specific support.Type of router, use its respective router
// implementation instead.
type Standard interface {
	support.Base
	support.Logs
	support.Capture
	support.Hostapd
	support.DHCP
	support.FrameSender
	support.IfaceManipulation
	support.VethBridgeBinding
	support.Bridge
	support.Veth
}
