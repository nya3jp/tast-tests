// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"chromiumos/tast/testing/hwdep"
)

// PublicBenchmarkAllowed returns the DUT model dependency of running public benchmark tests.
// Allowed hardware models will be white listed here.
func PublicBenchmarkAllowed() hwdep.Condition {
	return hwdep.Model(
		"ampton", "barla", "bluebird", "drawlat",
		"eve", "kled", "kohaku", "krane",
		"liara", "maple14", "pantheon", "pyke",
	)
}
