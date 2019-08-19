// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type appVerifier func(ctx context.Context, app *cca.App) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIIntent,
		Desc:         "Verifies if the camera intents fired from Android apps could be delivered and handled by CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinOrVividCamera},
		Pre:          arc.Booted(),
		Data:         []string{"cca_ui.js"},
	})
}

func CCAUIIntent(ctx context.Context, s *testing.State) {
	const (
		takePhotoAction         = "android.media.action.IMAGE_CAPTURE"
		recordVideoAction       = "android.media.action.VIDEO_CAPTURE"
		launchOnPhotoModeAction = "android.media.action.STILL_IMAGE_CAMERA"
		launchOnVideoModeAction = "android.media.action.VIDEO_CAMERA"
		photoMode               = "photo-mode"
		videoMode               = "video-mode"
		testPhotoURI            = "file:///sdcard/Download/test.png"
		testVideoURI            = "file:///sdcard/Download/test.mkv"
		defaultArcCameraPath    = "/run/arc/sdcard/write/emulated/0/DCIM/Camera"
	)

	var (
		testPhotoPattern = regexp.MustCompile(`^test\.png$`)
		testVideoPattern = regexp.MustCompile(`^test\.mkv$`)
	)

	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	checkIntent := func(action, uri string, verifier appVerifier) error {
		app, err := cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, func(tconn *chrome.Conn) error {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			testing.ContextLogf(ctx, "Testing action: %s", action)

			args := []string{"start", "-a", action}
			if uri != "" {
				args = append(args, "--eu", "output", uri)
			}

			output, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError)
			if err != nil {
				return err
			}
			testing.ContextLog(ctx, string(output))
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

	testing.ContextLog(ctx, "Starting intent behavior tests")

	if err := checkIntent(takePhotoAction, "", func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, photoMode); err != nil {
			return err
		}
		// Currently there is no way to evaluate the returned result of intent which is fired through
		// adb. We might rely on CTS Verifier to test such behavior.
		if err := checkCaptureResult(ctx, app, cca.Photo, true /* shouldReview */, "", nil); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, true); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for takePhotoAction: ", err)
	}

	if err := checkIntent(takePhotoAction, testPhotoURI, func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, photoMode); err != nil {
			return err
		}
		ccaSavedDir, err := app.GetSavedDir(ctx)
		if err != nil {
			return err
		}
		if err := checkCaptureResult(ctx, app, cca.Photo, true /* shouldReview */, ccaSavedDir, testPhotoPattern); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, true); err != nil {
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
		ccaSavedDir, err := app.GetSavedDir(ctx)
		if err != nil {
			return err
		}
		if err := checkCaptureResult(ctx, app, cca.Photo, false /* shouldReview */, ccaSavedDir, cca.PhotoPattern); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, false); err != nil {
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
		if err := checkCaptureResult(ctx, app, cca.Video, true /* shouldReview */, defaultArcCameraPath, cca.VideoPattern); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, true); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for recordVideoAction: ", err)
	}

	if err := checkIntent(recordVideoAction, testVideoURI, func(ctx context.Context, app *cca.App) error {
		if err := checkLandingMode(ctx, app, videoMode); err != nil {
			return err
		}
		ccaSavedDir, err := app.GetSavedDir(ctx)
		if err != nil {
			return err
		}
		if err := checkCaptureResult(ctx, app, cca.Video, true /* shouldReview */, ccaSavedDir, testVideoPattern); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, true); err != nil {
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
		ccaSavedDir, err := app.GetSavedDir(ctx)
		if err != nil {
			return err
		}
		if err := checkCaptureResult(ctx, app, cca.Video, false /* shouldReview */, ccaSavedDir, cca.VideoPattern); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, false); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for launchOnVideoModeAction: ", err)
	}

	// TODO(wtlee): Add intent cancelation tests

	// TODO(wtlee): Add intent instances coexistance tests
}

func checkLandingMode(ctx context.Context, app *cca.App, mode string) error {
	if result, err := app.GetState(ctx, mode); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.New("CCA does not land on correct mode")
	}
	return nil
}

func checkCaptureResult(ctx context.Context, app *cca.App, mode cca.Mode, shouldReview bool, dir string, filePattern *regexp.Regexp) error {
	startTime := time.Now()
	if mode == cca.Photo {
		testing.ContextLog(ctx, "Taking a photo")
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to click shutter button")
		}
	} else if mode == cca.Video {
		testing.ContextLog(ctx, "Recording a video")
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to click shutter button")
		}
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return err
		}
		testing.ContextLog(ctx, "Stopping a video")
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to click shutter button")
		}
	} else {
		return errors.Errorf("unrecognized mode: %s", mode)
	}

	if shouldReview {
		if err := app.ConfirmResult(ctx, true, mode); err != nil {
			return errors.Wrap(err, "failed to confirm result")
		}
	}

	// If no path is specified, skip the result checking part.
	if dir == "" || filePattern == nil {
		return nil
	}

	testing.ContextLog(ctx, "Checking capture result")
	if _, err := app.WaitForFileSaved(ctx, dir, filePattern, startTime); err != nil {
		return nil
	}
	return nil
}

func checkAutoCloseBehavior(ctx context.Context, cr *chrome.Chrome, shouldClose bool) error {
	// Sleeps for a while after capturing and then ensure CCA instance is automatically closed or not.
	testing.ContextLog(ctx, "Checking auto close behavior")
	if shouldClose {
		const timeout = 1 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if isExist, err := cca.InstanceExist(ctx, cr); err != nil {
				return testing.PollBreak(err)
			} else if !isExist {
				return nil
			}
			return errors.New("CCA instance is not automatically closed after capturing")
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			return err
		}
	} else {
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return err
		}
		if isExist, err := cca.InstanceExist(ctx, cr); err != nil {
			return err
		} else if !isExist {
			return errors.New("CCA instance is automatically closed after capturing")
		}
	}
	return nil
}
