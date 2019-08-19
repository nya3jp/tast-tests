// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type AppVerifier func(ctx context.Context, app *cca.App) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIIntent,
		Desc:         "Verifies if the camera intents fired from Android apps could be delivered and handled by CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.BuiltinCamera, "android", "chrome"},
		Data:         []string{"cca_ui.js"},
	})
}

// CCAUIIntent verifies whether the camera intents could be successfully delivered
// and handled by CCA.
func CCAUIIntent(ctx context.Context, s *testing.State) {
	const (
		takePhotoAction         = "android.media.action.IMAGE_CAPTURE"
		recordVideoAction       = "android.media.action.VIDEO_CAPTURE"
		launchOnPhotoModeAction = "android.media.action.STILL_IMAGE_CAMERA"
		launchOnVideoModeAction = "android.media.action.VIDEO_CAMERA"
		photoMode               = "photo-mode"
		videoMode               = "video-mode"
		testPhotoUri            = "file:///storage/emulated/0/Download/test.png"
		testPhotoPath           = "/home/chronos/user/Downloads/test.png"
		testVideoUri            = "file:///storage/emulated/0/Download/test.mkv"
		testVideoPath           = "/home/chronos/user/Downloads/test.mkv"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// To keep background page of CCA alive when waiting for ARC++ initialization,
	// we need to connect it first so that it won't idle.
	ccaID := "hfhhnacclhffhdffklopdkcgdhifgngh"
	bgURL := chrome.ExtensionBackgroundPageURL(ccaID)
	dummyConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatal("Failed to construct dummy connection to CCA background")
	}
	defer dummyConn.Close()
	defer dummyConn.CloseTarget(ctx)

	// Initialize ARC++
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		s.Fatal("Failed to get output dir")
	}
	a, err := arc.New(ctx, outDir)
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	checkIntent := func(action, uri string, verifier AppVerifier) error {
		app, err := cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, func() error {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			testing.ContextLogf(ctx, "Testing: %s", action)

			args := []string{"start", "-a", action}
			if uri != "" {
				args = append(args, "--eu", "output", uri)
			}

			if output, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError); err != nil {
				return err
			} else {
				testing.ContextLog(ctx, string(output))
			}
			return nil
		})
		defer app.Close(ctx)

		if err != nil {
			return err
		}

		if err := app.WaitForVideoActive(ctx); err != nil {
			return err
		}

		if err := verifier(ctx, app); err != nil {
			return err
		}
		return nil
	}

	testing.ContextLog(ctx, "Starting intent tests")
	if err := checkIntent(takePhotoAction, "", func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, photoMode); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for takePhotoAction: ", err)
	}

	if err := checkIntent(takePhotoAction, testPhotoUri, func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, photoMode); err != nil {
			return err
		}
		if err := checkTakePhotoResult(ctx, app, testPhotoPath); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for takePhotoAction: ", err)
	}

	if err := checkIntent(launchOnPhotoModeAction, "", func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, photoMode); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for launchOnPhotoModeAction: ", err)
	}

	if err := checkIntent(recordVideoAction, "", func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, videoMode); err != nil {
			return nil
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for recordVideoAction: ", err)
	}

	if err := checkIntent(recordVideoAction, testVideoUri, func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, videoMode); err != nil {
			return err
		}
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			return err
		}
		if err := checkRecordVideoResult(ctx, app, testVideoPath); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for recordVideoAction: ", err)
	}

	if err := checkIntent(launchOnVideoModeAction, "", func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, videoMode); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for launchOnVideoModeAction: ", err)
	}
}

func checkLandingMode(ctx context.Context, app *cca.App, mode string) error {
	if result, err := app.GetState(ctx, mode); err != nil {
		return errors.Wrap(err, "Failed to check state")
	} else if !result {
		return errors.Errorf("CCA does not land on correct mode")
	}
	return nil
}

func checkTakePhotoResult(ctx context.Context, app *cca.App, path string) error {
	testing.ContextLog(ctx, "Taking a photo")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "Failed to click shutter button")
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Capturing hasn't ended")
	}

	testing.ContextLog(ctx, "Checking the test image")
	const timeout = 5 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		return errors.New("No matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "No matching output file found after %v", timeout)
	}

	testing.ContextLog(ctx, "Removing the test image")
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "Failed to delete the test image")
	}
	return nil
}

func checkRecordVideoResult(ctx context.Context, app *cca.App, path string) error {
	testing.ContextLog(ctx, "Recording a video")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "Failed to click shutter button")
	}
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Stopping a video")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "Failed to click shutter button")
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Capturing hasn't ended")
	}

	testing.ContextLog(ctx, "Checking the test video")
	const timeout = 5 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		return errors.New("No matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "No matching output file found after %v", timeout)
	}

	testing.ContextLog(ctx, "Removing the test video")
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "Failed to delete the test video")
	}
	return nil
}
