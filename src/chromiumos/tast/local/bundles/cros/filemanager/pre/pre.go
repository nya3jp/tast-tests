// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains the preconditions used by the filemanager tests.
package pre

import (
	"chromiumos/tast/local/drivefs"
)

var drivefsGaia = &drivefs.GaiaVars{
	UserVar: "filemanager.user",
	PassVar: "filemanager.password",
}

// DriveFsStarted is a precondition that logs in a real user and waits for drive to stabilise.
var DriveFsStarted = drivefs.NewPrecondition("drivefs_started", drivefsGaia)
