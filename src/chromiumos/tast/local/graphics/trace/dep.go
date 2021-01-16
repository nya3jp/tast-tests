// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package trace

import (
	"chromiumos/tast/testing/hwdep"
)

// modelAllowlist is a list of model that is targeted to be able to reliably work in the lab for testing.
var modelAllowlist = []string{"atlas", "eve", "drallion", "nocturne"}

// HwDepsStable is hardwareDeps condition that stable to run trace tests.
var HwDepsStable = hwdep.D(hwdep.Model(modelAllowlist...))

// HwDepsUnstable is hardwareDeps condition that unstable to run trace tests.
var HwDepsUnstable = hwdep.D(hwdep.SkipOnModel(modelAllowlist...))
