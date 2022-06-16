// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

type cameraFacing int

const (
	userFacing cameraFacing = iota
	backFacing
)

type cameraStressTestParams struct {
	facing         cameraFacing
	iter           int
	isCaptureImage bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUICaptureOperation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies capturing of images, video using user-facing and back-facing camera stress test",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
		Params: []testing.Param{{
			Name: "user_facing_image_quick",
			Val:  cameraStressTestParams{userFacing, 2, true},
		}, {
			Name:    "user_facing_image_bronze",
			Val:     cameraStressTestParams{userFacing, 360, true},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "user_facing_image_silver",
			Val:     cameraStressTestParams{userFacing, 540, true},
			Timeout: 8 * time.Minute,
		}, {
			Name:    "user_facing_image_gold",
			Val:     cameraStressTestParams{userFacing, 720, true},
			Timeout: 12 * time.Minute,
		}, {
			Name:    "user_facing_video_quick",
			Val:     cameraStressTestParams{userFacing, 2, false},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "user_facing_video_bronze",
			Val:     cameraStressTestParams{userFacing, 360, false},
			Timeout: 75 * time.Minute,
		}, {
			Name:    "user_facing_video_silver",
			Val:     cameraStressTestParams{userFacing, 540, false},
			Timeout: 118 * time.Minute,
		}, {
			Name:    "user_facing_video_gold",
			Val:     cameraStressTestParams{userFacing, 720, false},
			Timeout: 150 * time.Minute,
		}, {
			Name:    "env_facing_image_bronze",
			Val:     cameraStressTestParams{backFacing, 360, true},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "env_facing_image_silver",
			Val:     cameraStressTestParams{backFacing, 540, true},
			Timeout: 8 * time.Minute,
		}, {
			Name:    "env_facing_image_gold",
			Val:     cameraStressTestParams{backFacing, 720, true},
			Timeout: 12 * time.Minute,
		}, {
			Name:    "env_facing_video_bronze",
			Val:     cameraStressTestParams{backFacing, 360, false},
			Timeout: 75 * time.Minute,
		}, {
			Name:    "env_facing_video_silver",
			Val:     cameraStressTestParams{backFacing, 540, false},
			Timeout: 118 * time.Minute,
		}, {
			Name:    "env_facing_video_gold",
			Val:     cameraStressTestParams{backFacing, 720, false},
			Timeout: 150 * time.Minute,
		}}})
}

func CCAUICaptureOperation(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cr := s.FixtValue().(cca.FixtureData).Chrome
	app := s.FixtValue().(cca.FixtureData).App()
	defer app.Close(ctxForCleanUp)
	defer cca.ClearSavedDir(ctxForCleanUp, cr)

	testParam := s.Param().(cameraStressTestParams)

	const (
		userFacingCamera = cca.FacingFront
		envFacingCamera  = cca.FacingBack
	)

	iter := testParam.iter
	switch testParam.facing {
	case userFacing:
		// Check whether user facing camera switched.
		if err := setCameraFacing(ctx, app, userFacingCamera); err != nil {
			s.Fatal("Failed to set camera facing: ", err)
		}

	case backFacing:
		// Check whether back facing camera switched.
		if err := setCameraFacing(ctx, app, envFacingCamera); err != nil {
			s.Fatal("Failed to set camera facing: ", err)
		}
	}

	for i := 1; i <= testParam.iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		if testParam.isCaptureImage {
			if err := captureImage(ctx, app); err != nil {
				s.Fatalf("Failed to capture image using %q camera: %v", testParam.facing, err)
			}
		} else {
			if err := captureVideo(ctx, app); err != nil {
				s.Fatalf("Failed to capture video using %q camera: %v", testParam.facing, err)
			}
		}
		// Clear captured files for every 50 iterations.
		if i%50 == 0 {
			if err := cca.ClearSavedDir(ctx, cr); err != nil {
				s.Errorf("Failed to clear files at %d iteration while on user-facing: %v", i, err)
			}
		}
	}
}

// captureImage captures a single photo.
func captureImage(ctx context.Context, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		return errors.Wrap(err, "failed to switch to photo mode")
	}
	_, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to take photo")
	}
	return nil
}

// captureVideo captures a video of 10s duration.
func captureVideo(ctx context.Context, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for video to active")
	}
	_, err := app.RecordVideo(ctx, cca.TimerOff, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to record video")
	}
	return nil
}

// setCameraFacing sets the camera facing to wantFacing.
func setCameraFacing(ctx context.Context, app *cca.App, wantFacing cca.Facing) error {
	gotFacing, err := app.GetFacing(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get camera facing")
	}
	if gotFacing != wantFacing {
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			return errors.Wrap(err, "failed to switch camera")
		}
		gotFacing, err = app.GetFacing(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get facing after switching")
		}
		if gotFacing != wantFacing {
			return errors.Errorf("failed to switch to camera: got %q; want %q", gotFacing, wantFacing)
		}
	}
	return nil
}
