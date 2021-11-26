// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"chromiumos/tast/testing/hwdep"
)

// PerfCUJBasicDevices returns list of DUT model in basic tier.
// Allowed basic hardware models will be alowed-listed here.
func PerfCUJBasicDevices() hwdep.Condition {
	return hwdep.Model("babymako", "barla", "druwl", "kasumi", "kitefin", "liara", "treeya")
}

// PerfCUJPlusDevices returns list of DUT model in plus tier.
// Allowed plus hardware models will be alowed-listed here.
func PerfCUJPlusDevices() hwdep.Condition {
	return hwdep.Model("coachz", "burnet", "juniper", "kenzo", "limozeen", "sarien", "sycamore")
}

// PerfCUJPremiumDevices returns list of DUT model in premium tier.
// Allowed premium hardware models will be alowed-listed here.
func PerfCUJPremiumDevices() hwdep.Condition {
	return hwdep.Model("volta", "vilboz", "dratini", "akemi", "dratini", "dragonair", "dooly", "drobit", "wyvern")
}
