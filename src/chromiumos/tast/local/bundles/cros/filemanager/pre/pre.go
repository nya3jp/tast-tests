// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// TODO: migrate filemanager preconditions to fixtures

// Package pre contains the preconditions used by the filemanager tests.
package pre

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/drivefs"
)

var drivefsGaia = &drivefs.GaiaVars{
	UserVar: "filemanager.user",
	PassVar: "filemanager.password",
}

// DriveFsStarted is a precondition that logs in a real user and waits for drive to stabilise.
var DriveFsStarted = drivefs.NewPrecondition("filemanager_drivefs_started", drivefsGaia)

// DriveFsWithDssPinning is the same as above, except with the flag to enable pinning of Docs/Sheets/Slides files.
var DriveFsWithDssPinning = drivefs.NewPrecondition("filemanager_drivefs_with_dss_pinning", drivefsGaia, chrome.EnableFeatures("DriveFsBidirectionalNativeMessaging"))
