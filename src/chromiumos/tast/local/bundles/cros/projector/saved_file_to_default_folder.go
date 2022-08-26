// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SavedFileToDefaultFolder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Creates a screencast and verifies it is saved at the default screencast storage folder in DriveFS",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
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

	cleanup, err := projector.SetUpProjectorApp(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up Projector app: ", err)
	}
	defer cleanup(ctxForCleanUp)

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

	containerFolderPath := driveFsClient.MyDrivePath(filepath.Join("Screencast recordings", screencastInfo.Name))
	mediaFile := filepath.Join(containerFolderPath, screencastInfo.Name+".webm")
	metadataFile := filepath.Join(containerFolderPath, screencastInfo.Name+".projector")
	thumbnailFile := filepath.Join(containerFolderPath, "thumbnail.png")
	screencastFiles := []string{mediaFile, metadataFile, thumbnailFile}
	for _, screencastFile := range screencastFiles {
		if _, err := os.Stat(screencastFile); err != nil {
			s.Fatal("Failed to locate screencast file in default folder: ", err)
		}
	}

}
