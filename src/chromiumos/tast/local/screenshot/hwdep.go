// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"chromiumos/tast/testing/hwdep"
)

// SubpixelAAPlatform is a list of platforms that have subpixel antialiasing enabled.
var SubpixelAAPlatform = []string{
	"asuka",
	"banjo",
	"banon",
	"candy",
	"celes",
	"chell",
	"edgar",
	"enguarde",
	"gnawty",
	"kip",
	"lars",
	"orco",
	"puff",
	"reks",
	"relm",
	"sand",
	"sentry",
	"sumo",
	"swanky",
	"veyron_fievel",
	"veyron_tiger",
	"winky",
}

// ScreenshotStableCond is a hardware condition that only runs a test on models
// that can run screenshot tests without known flakiness issues.
var ScreenshotStableCond = hwdep.SkipOnPlatform(SubpixelAAPlatform...)

// ScreenshotStable is a hardware dependency that only runs a test on models
// that can run screenshot tests without known flakiness issues.
var ScreenshotStable = hwdep.D(ScreenshotStableCond)
