// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"chromiumos/tast/testing/hwdep"
)

// ThunderboltSupportedDevices returns list of DUT model that supports thunderbolt.
// Allowed thunderbolt hardware models will be allowed-listed here.
func ThunderboltSupportedDevices() hwdep.Condition {
	return hwdep.Model("voxel", "redrix", "brya")
}
