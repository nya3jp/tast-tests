// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package trace

import (
	"chromiumos/tast/testing/hwdep"
)

// modelWhitelist is a list of model that is targeted to be able to reliably work in the lab for testing.
var modelWhitelist = []string{"atlas", "coral", "drallion", "eve", "fizz", "kalista", "kevin", "nautilus", "pyro", "sand", "sarien", "scarlet", "setzer", "snappy", "soraka", "terra", "ultima", "wizpig"}

// HwDepsStable is hardwareDeps condition that stable to run trace tests.
var HwDepsStable = hwdep.D(hwdep.Model(modelWhitelist...))

// HwDepsUnstable is hardwareDeps condition that unstable to run trace tests.
var HwDepsUnstable = hwdep.D(hwdep.SkipOnModel(modelWhitelist...))
