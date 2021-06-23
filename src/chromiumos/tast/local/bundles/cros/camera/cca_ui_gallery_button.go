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
	"chromiumos/tast/errors"
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

	backgroundImageAttr := "background-image"

	// 1. Take a photo and the gallery button should be updated.
	thumbnail, err := app.Style(ctx, cca.GalleryButton, backgroundImageAttr)
	if err != nil {
		s.Error("Failed to get the thumbnail of the gallery button: ", err)
	}
	infos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Error("Failed to take a photo: ", err)
	}
	photoPath, err := app.FilePathInSavedDirs(ctx, infos[0].Name())
	if err != nil {
		s.Error("Failed to get captured photo path: ", err)
	}
	newThumbnail, err := app.Style(ctx, cca.GalleryButton, backgroundImageAttr)
	if err != nil {
		s.Error("Failed to get thumbnail of the gallery button after capture: ", err)
	}
	if thumbnail == newThumbnail {
		s.Error("Thumbnail is not updated after capture")
	}
	thumbnail = newThumbnail

	// 2. Click the gallery button and the Backlight app should be launched in 10 seconds.
	if err := app.Click(ctx, cca.GalleryButton); err != nil {
		s.Error("Failed to click the gallery button: ", err)
	}
	checkMediaAppPrefix := func(t *target.Info) bool {
		url := "chrome://media-app"
		return strings.HasPrefix(t.URL, url)
	}
	var mediaAppTargetID target.ID
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		mediaAppTargets, err := cr.FindTargets(ctx, checkMediaAppPrefix)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(mediaAppTargets) == 0 {
			return errors.New("Media app should be launched")
		} else if len(mediaAppTargets) > 1 {
			return errors.New("More than one media app is launched")
		}
		mediaAppTargetID = mediaAppTargets[0].TargetID
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to launch media app within given time after clicking the gallery button: ", err)
	}
	if err := cr.CloseTarget(ctx, mediaAppTargetID); err != nil {
		s.Error("Failed to close the media app: ", err)
	}

	// 3. Delete the file just captured and the gallery button should be updated in 10 seconds.
	if err := os.Remove(photoPath); err != nil {
		s.Error("Failed to remove captured photo: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newThumbnail, err = app.Style(ctx, cca.GalleryButton, backgroundImageAttr)
		if err != nil {
			return testing.PollBreak(err)
		}
		if thumbnail == newThumbnail {
			return errors.New("Thumbnail is not updated")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to update thumbnail of the gallery button after the file it points to is deleted: ", err)
	}
}
