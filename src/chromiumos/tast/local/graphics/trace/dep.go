// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package trace

import (
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing/hwdep"
)

// modelBlackList is a list of model that are too flaky for the CQ.
// This is in addition to crostini.modelBlacklist.
var modelBlackList = append(crostini.ModelBlacklist,
	// Platform jecht
	"guado",
	"jecht",
	"rikku",
	"tidus",
)

// HwDepsStable is hardwareDeps condition that stable to run trace tests.
var HwDepsStable = hwdep.D(hwdep.SkipOnModel(modelBlackList...))

// HwDepsUnstable is hardwareDeps condition that unstable to run trace tests.
var HwDepsUnstable = hwdep.D(hwdep.Model(modelBlackList...))
