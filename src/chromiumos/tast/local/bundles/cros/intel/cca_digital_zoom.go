// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

type functionality int

const (
	photoTaking functionality = iota
	videoRecording
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCADigitalZoom,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Digital Zoom in image preview/video preview and capture image/record video",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"camera_app", "chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "ccaLaunchedWithFakeCamera",
		Params: []testing.Param{{
			Name: "photo",
			Val:  photoTaking,
		}, {
			Name: "video",
			Val:  videoRecording,
		}},
	})
}

func CCADigitalZoom(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	testFunction := s.Param().(functionality)

	if testFunction == photoTaking {
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Fatal("Failed to switch to photo mode: ", err)
		}
		if err := zoomInOutTakePhoto(ctx, app); err != nil {
			s.Fatal("Failed to perform photo operation: ", err)
		}
	} else if testFunction == videoRecording {
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			s.Fatal("Failed to switch to photo mode: ", err)
		}
		if err := zoomInOutRecordVideo(ctx, app); err != nil {
			s.Fatal("Failed to perform video operation: ", err)
		}
	}
}

// zoomInOutRecordVideo zooms in/out and record video.
func zoomInOutRecordVideo(ctx context.Context, app *cca.App) error {
	if err := app.Click(ctx, cca.OpenPTZPanelButton); err != nil {
		return errors.Wrap(err, "failed to open ptz panel")
	}
	const zoomFactor int = 3
	for i := 0; i < zoomFactor; i++ {
		if err := app.ClickPTZButton(ctx, cca.ZoomInButton); err != nil {
			return errors.Wrap(err, "failed to click zoom in button")
		}
		// Record video after increasing zoom.
		if _, err := app.RecordVideo(ctx, cca.TimerOff, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to record video")
		}
	}

	for i := 0; i < zoomFactor; i++ {
		if err := app.ClickPTZButton(ctx, cca.ZoomOutButton); err != nil {
			return errors.Wrap(err, "failed to click zoom out button")
		}
		// Record video after decreasing zoom.
		if _, err := app.RecordVideo(ctx, cca.TimerOff, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to record video")
		}
	}
	return nil
}

// zoomInOutTakePhoto zooms in/out and take photo.
func zoomInOutTakePhoto(ctx context.Context, app *cca.App) error {
	if err := app.Click(ctx, cca.OpenPTZPanelButton); err != nil {
		return errors.Wrap(err, "failed to open ptz panel")
	}

	const zoomFactor int = 3
	for i := 0; i < zoomFactor; i++ {
		if err := app.ClickPTZButton(ctx, cca.ZoomInButton); err != nil {
			return errors.Wrap(err, "failed to click zoom in button")
		}
		// Take photo after increasing zoom.
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			return errors.Wrap(err, "failed to take photo")
		}
	}

	for i := 0; i < zoomFactor; i++ {
		if err := app.ClickPTZButton(ctx, cca.ZoomOutButton); err != nil {
			return errors.Wrap(err, "failed to click zoom out button")
		}
		// Take photo after decreasing zoom.
		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			return errors.Wrap(err, "failed to take photo")
		}
	}

	return nil
}
