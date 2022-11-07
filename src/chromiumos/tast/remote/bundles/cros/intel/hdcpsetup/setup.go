// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hdcpsetup

import (
	"chromiumos/tast/testing/hwdep"
)

// PerfHDCPDevices returns list of DUT model which supports HDCP.
// HDCP stands for High-bandwidth Digital Content Protection.
// Allowed hardware models are listed below.
func PerfHDCPDevices() hwdep.Condition {
	return hwdep.Model("volteer", "voxel", "redrix", "brya")
}
