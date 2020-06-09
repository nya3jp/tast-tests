// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains the preconditions used by the arcappcompat tests.
package pre

import (
	"chromiumos/tast/local/drivefs"
)

var drivefsGaia = &drivefs.GaiaVars{
	UserVar: "drivefs.username",
	PassVar: "drivefs.password",
}

// DrivefsStarted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the app compat credentials, and opt-in to the Play Store.
var DrivefsStarted = drivefs.NewPrecondition("drivefs_started", drivefsGaia)
