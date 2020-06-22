// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Drivefs,
		Desc: "Verifies that drivefs mounts on sign in",
		Contacts: []string{
			"dats@chromium.org",
			"austinct@chromium.org",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Attr: []string{
			"group:mainline",
		},
		Vars: []string{
			"platform.Drivefs.user",     // GAIA username.
			"platform.Drivefs.password", // GAIA password.
		},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("nyan_kitty")), // TODO(crbug.com/1097615): Remove when test fixed on nyan_kitty.
	})
}

func Drivefs(ctx context.Context, s *testing.State) {
	user := s.RequiredVar("platform.Drivefs.user")
	password := s.RequiredVar("platform.Drivefs.password")

	// Sign in a real user.
	cr, err := chrome.New(
		ctx,
		chrome.ARCDisabled(),
		chrome.Auth(user, password, ""),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")
	dir, err := os.Stat(drivefsRoot)
	if err != nil {
		s.Fatal("Could not stat ", drivefsRoot, ": ", err)
	}
	if !dir.IsDir() {
		s.Fatal("Could not find root folder inside ", mountPath, ": ", err)
	}

	// Check for team_drives too.
	drivefsTeamDrives := path.Join(mountPath, "team_drives")
	dir, err = os.Stat(drivefsTeamDrives)
	if err != nil {
		s.Fatal("Could not stat ", drivefsTeamDrives, ": ", err)
	}
	if !dir.IsDir() {
		s.Fatal("Could not find team_drives folder inside ", mountPath, ": ", err)
	}
}
