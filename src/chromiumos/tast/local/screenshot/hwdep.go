// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package screenshot

import (
	"chromiumos/tast/testing/hwdep"
)

// SubpixelAAUnstablePlatforms is a list of platforms that have subpixel
// antialiasing enabled, but produce slightly differentt looking (but still
// correct) images to others.
// TODO(b/191103841): Attempt to enable it on these platforms
var SubpixelAAUnstablePlatforms = []string{
	"chell",
	"puff",
	"sumo",
	"veyron_fievel",
	"veyron_tiger",
}

// ScreenshotStableCond is a hardware condition that only runs a test on models
// that can run screenshot tests without known flakiness issues.
var ScreenshotStableCond = hwdep.SkipOnPlatform(SubpixelAAUnstablePlatforms...)

// ScreenshotStable is a hardware dependency that only runs a test on models
// that can run screenshot tests without known flakiness issues.
var ScreenshotStable = hwdep.D(ScreenshotStableCond)
