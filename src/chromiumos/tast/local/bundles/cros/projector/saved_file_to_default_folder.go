// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SavedFileToDefaultFolder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches the Projector app and goes through the new screencast creation flow with annotator",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      12 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func SavedFileToDefaultFolder(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 4*time.Minute)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn
	cr := s.FixtValue().(*projector.FixtData).Chrome
	driveFsClient, err := drivefs.NewDriveFs(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("Drivefs fully started")

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	if err := projector.DismissOnboardingDialog(ctx, tconn); err != nil {
		s.Fatal("Failed to close the onboarding dialog: ", err)
	}

	// We need to clean up any screencasts after the test to
	// prevent taking up Drive quota over time.
	defer projector.DeleteScreencastItems(ctxForCleanUp, tconn)
	if err := projector.LaunchCreationFlow(ctx, tconn, false /*launchAnnotator*/); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}

	// Verifies Screencast saved to right location.
	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)
	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	screencastTitle := nodewith.Role(role.StaticText).Ancestor(nodewith.ClassName("screencast-title").Ancestor(screencastItem))
	if err := ui.WaitUntilExists(screencastItem)(ctx); err != nil {
		s.Fatal("Failed to wait for the screencast item to show: ", err)
	}
	screencastInfo, err := ui.Info(ctx, screencastTitle)
	if err != nil {
		s.Fatal("Failed to get screencast title info: ", err)
	}

	testFilePath := driveFsClient.MyDrivePath(filepath.Join("Screencast recordings", screencastInfo.Name))
	screencastFileExistence := map[string]bool{
		".projector": false,
		".png":       false,
		".webm":      false,
	}
	screencastFileCount := 0
	if err := filepath.Walk(testFilePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == testFilePath {
				return nil
			}
			return errors.New("unknown directory")
		}
		ext := filepath.Ext(path)
		existence, isScreencastExt := screencastFileExistence[ext]
		if !isScreencastExt {
			return errors.Errorf("%s is not a valid screencast extension", ext)
		}
		if existence {
			return errors.Errorf("should not have multiple %s file", ext)
		}
		screencastFileExistence[ext] = true
		screencastFileCount++
		return nil
	}); err != nil || screencastFileCount != 3 {
		s.Fatalf("Invalid screencast with container folder %s and count of valid screencast file is %d: %v", testFilePath, screencastFileCount, err)
	}
}
