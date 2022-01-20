// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package router

import (
	"chromiumos/tast/remote/wificell/router/common/support"
)

// Router contains the functionality the standard WiFi testing router controller should support.
// Use this in tests if you are not specifically testing with a router that has different support
type Router interface {
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
