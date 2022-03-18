// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wlan provides the information of the wlan device.
package wlan

import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/wlan"
)

const (
	intelVendorNum   = "0x8086"
	support160MHz    = '0'
	supportOnly80MHz = '2'
)

// LogBandwidthSupport logs info about the device bandwidth support.
// For now, it only works for Intel devices.
func LogBandwidthSupport(ctx context.Context, dev *wlan.DevInfo) {
	if dev.Vendor != intelVendorNum {
		return
	}
	if len(dev.Subsystem) < 4 {
		return
	}
	if dev.Subsystem[3] == support160MHz {
		testing.ContextLog(ctx, "Bandwidth Support: Supports 160 MHz Bandwidth")
	} else if dev.Subsystem[3] == supportOnly80MHz {
		testing.ContextLog(ctx, "Bandwidth Support: Supports only 80 MHz Bandwidth")
	} else {
		testing.ContextLog(ctx, "Bandwidth Support: Doesn't support (80 MHz , 160 MHz) Bandwidth")
	}
}
