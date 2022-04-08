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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGalleryButton,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that gallery button related logic works expectedly in CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIGalleryButton(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	cr := s.FixtValue().(cca.FixtureData).Chrome
	backgroundImageAttr := "background-image"

	// 1. Take a photo and the gallery button should be updated.
	coverURL := func() (string, error) {
		url, err := app.AttributeWithIndex(ctx, cca.GalleryButtonCover, 0, "src")
		if cca.IsUINotExist(err) {
			// Fallback to legacy UI before https://crrev.com/c/3340974.
			url, err = app.Style(ctx, cca.GalleryButton, backgroundImageAttr)
		}
		if err != nil {
			return "", err
		}
		return url, nil
	}
	url, err := coverURL()
	if err != nil {
		s.Error("Failed to get the url of the gallery button: ", err)
	}
	infos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Error("Failed to take a photo: ", err)
	}
	photoPath, err := app.FilePathInSavedDir(ctx, infos[0].Name())
	if err != nil {
		s.Error("Failed to get captured photo path: ", err)
	}
	newURL, err := coverURL()
	if err != nil {
		s.Error("Failed to get url of the gallery button after capture: ", err)
	}
	if url == newURL {
		s.Error("Cover is not updated after capture")
	}

	// 2. Click the gallery button and the Backlight app should be launched in 10 seconds.
	if err := app.Click(ctx, cca.GalleryButton); err != nil {
		s.Error("Failed to click the gallery button: ", err)
	}
	checkMediaAppPrefix := func(t *target.Info) bool {
		// TODO(b/204528998): Change back to chrome://media-app once the
		// underlying issue is solved so we won't hit the race condition.
		url := "chrome-untrusted://media-app"
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
	url = newURL
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newURL, err = coverURL()
		if err != nil {
			return testing.PollBreak(err)
		}
		if url == newURL {
			return errors.New("Cover is not updated")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to update cover of the gallery button after the file it points to is deleted: ", err)
	}
}
