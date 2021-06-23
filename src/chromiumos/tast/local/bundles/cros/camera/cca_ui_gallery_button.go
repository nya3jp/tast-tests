// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGalleryButton,
		Desc:         "Verifies that gallery button related logic works expectedly in CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIGalleryButton(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	// 1. Take a photo and the gallery button should be updated.
	backgroundURL, err := app.BackgroundURL(ctx, cca.GalleryButton)
	if err != nil {
		s.Error("Failed to get background URL of the gallery button: ", err)
	}
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		s.Error("Failed to swith to photo mode: ", err)
	}
	infos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Error("Failed to take a photo: ", err)
	}
	photoPath, err := app.FilePathInSavedDirs(ctx, infos[0].Name())
	if err != nil {
		s.Error("Failed to get captured photo path: ", err)
	}
	newBackgroundURL, err := app.BackgroundURL(ctx, cca.GalleryButton)
	if err != nil {
		s.Error("Failed to get background URL of the gallery button after capture: ", err)
	}
	if backgroundURL == newBackgroundURL {
		s.Error("Background image is not updated after capture: ", err)
	}
	backgroundURL = newBackgroundURL

	// 2. Click the gallery button and the Backlight app should be launched in 3 seconds.
	if err := app.Click(ctx, cca.GalleryButton); err != nil {
		s.Error("Failed to click the gallery button: ", err)
	}
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Error("Failed to sleep for 3 seconds: ", err)
	}
	checkMediaAppPrefix := func(t *target.Info) bool {
		url := "chrome://media-app"
		return strings.HasPrefix(t.URL, url)
	}
	mediaAppTargets, err := cr.FindTargets(ctx, checkMediaAppPrefix)
	if err != nil {
		s.Error("Failed to check media app existence: ", err)
	}
	if len(mediaAppTargets) == 0 {
		s.Error("Media app should be launched after clicking gallery button")
	} else if len(mediaAppTargets) > 1 {
		s.Error("More than one media app is launched")
	}
	if err := cr.CloseTarget(ctx, mediaAppTargets[0].TargetID); err != nil {
		s.Error("Failed to close the media app: ", err)
	}

	// 3. Delete the file just captured and the gallery button should be updated in 3 seconds.
	if err := os.Remove(photoPath); err != nil {
		s.Error("Failed to remove captured photo: ", err)
	}
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Error("Failed to sleep for 3 seconds: ", err)
	}
	newBackgroundURL, err = app.BackgroundURL(ctx, cca.GalleryButton)
	if err != nil {
		s.Error("Failed to get background URL of the gallery button after capture: ", err)
	}
	if backgroundURL == newBackgroundURL {
		s.Error("Background image is not updated after the file it points to is deleted")
	}
}
