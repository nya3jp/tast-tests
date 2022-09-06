// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCaptureWithResolutions,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify video recording with maximum resolution and different aspect ratios (user facing, world facing camera)",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Timeout:      20 * time.Minute,
		Fixture:      "ccaLaunched",
		Params: []testing.Param{{
			Name: "front",
			Val:  cca.FacingFront,
		}, {
			Name: "back",
			Val:  cca.FacingBack,
		}},
	})
}

func VideoCaptureWithResolutions(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	facing := s.Param().(cca.Facing)

	if facing == cca.FacingBack {
		numCameras, err := app.GetNumOfCameras(ctx)
		if err != nil {
			s.Fatal("Failed to get number of cameras: ", err)
		}
		// since DUT has single camera skipping world-facing-camera test.
		if numCameras <= 1 {
			s.Fatal("DUT don't have world facing camera")
		}
	}

	if curFacing, err := app.GetFacing(ctx); err != nil {
		s.Fatal("Failed to get facing: ", err)
	} else if curFacing != facing {
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Failed to switch camera: ", err)
		}
		if err := app.CheckFacing(ctx, facing); err != nil {
			s.Fatalf("Failed to switch to the target camera %v: %v", facing, err)
		}
	}

	if err := iterateResolutionAndRecord(ctx, app); err != nil {
		s.Fatal("Failed to iterate resolution and record video: ", err)
	}

}

// iterateResolutionAndRecord iterates through available resolutions and records video using cca app.
func iterateResolutionAndRecord(ctx context.Context, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}

	if err := cca.MainMenu.Open(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open main menu")
	}
	defer cca.MainMenu.Close(ctx, app)

	if err := cca.VideoResolutionMenu.Open(ctx, app); err != nil {
		return errors.Wrap(err, "failed to open resolution main menu")
	}
	defer cca.VideoResolutionMenu.Close(ctx, app)

	facing, err := app.GetFacing(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get facing")
	}

	resolutionOptions := cca.FrontVideoResolutionOptions
	if facing == cca.FacingBack {
		resolutionOptions = cca.BackVideoResolutionOptions
	}

	numOptions, err := app.CountUI(ctx, resolutionOptions)
	if err != nil {
		return errors.Wrap(err, "failed to count the resolution options")
	}

	for index := 0; index < numOptions; index++ {
		if err := app.ClickWithIndex(ctx, resolutionOptions, index); err != nil {
			return errors.Wrap(err, "failed to click on resolution item")
		}
		if err := recordVideoAndCheckProfile(ctx, app); err != nil {
			return errors.Wrap(err, "failed to record video and verify profile")
		}
	}
	return nil
}

// recordVideoAndCheckProfile records a video and checks profile of video file recorded by CCA.
func recordVideoAndCheckProfile(ctx context.Context, app *cca.App) error {
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for video to active")
	}
	// Recording video for 5 minutes.
	info, err := app.RecordVideo(ctx, cca.TimerOff, 5*time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to record video")
	}
	path, err := app.FilePathInSavedDir(ctx, info.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get file path")
	}
	return cca.CheckVideoProfile(path, cca.ProfileH264High)
}
