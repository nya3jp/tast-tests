// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"io/ioutil"
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

type appVerifier func(ctx context.Context, app *cca.App) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIIntent,
		Desc:         "Verifies if the camera intents fired from Android apps could be delivered and handled by CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.BuiltinCamera, "android", "chrome"},
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
		testPhotoPath           = "/home/chronos/user/Downloads/test.png"
		testVideoURI            = "file:///sdcard/Download/test.mkv"
		testVideoPath           = "/home/chronos/user/Downloads/test.mkv"
		defaultVideoFolderPath  = "/run/arc/sdcard/write/emulated/0/DCIM/Camera"
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
		if err := checkTakePhotoResult(ctx, app, true /* shouldReview */, ""); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, false); err != nil {
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
		if err := checkTakePhotoResult(ctx, app, true /* shouldReview */, testPhotoPath); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, false); err != nil {
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
		if err := checkTakePhotoResult(ctx, app, false /* shouldReview */, ""); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, true); err != nil {
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
		if err := checkRecordVideoResult(ctx, app, true /* shouldReview */, true /* isFolder */, defaultVideoFolderPath); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, false); err != nil {
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
		if err := checkRecordVideoResult(ctx, app, true /* shouldReview */, false /* isFolder */, testVideoPath); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, false); err != nil {
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
		if err := checkRecordVideoResult(ctx, app, false /* shouldReview */, false /* isFolder */, ""); err != nil {
			return err
		}
		if err := checkCCAInstanceExist(ctx, cr, true); err != nil {
			return err
		}
		return nil
	}); err != nil {
		s.Fatal("Failed for launchOnVideoModeAction: ", err)
	}

	testing.ContextLog(ctx, "Starting intent cancelation tests")
	// TODO(wtlee)

	testing.ContextLog(ctx, "Starting CCA instances coexist tests")
	// TODO(wtlee)
}

func checkLandingMode(ctx context.Context, app *cca.App, mode string) error {
	if result, err := app.GetState(ctx, mode); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.New("CCA does not land on correct mode")
	}
	return nil
}

func checkTakePhotoResult(ctx context.Context, app *cca.App, shouldReview bool, path string) error {
	testing.ContextLog(ctx, "Taking a photo")
	if err := app.ClickShutter(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter button")
	}

	if shouldReview {
		if err := app.WaitForState(ctx, "review-result", true); err != nil {
			return errors.Wrap(err, "does not enter review result state")
		}
		if err := app.CheckConfirmUIExist(ctx, false); err != nil {
			return errors.Wrap(err, "check confirm UI failed")
		}
		if err := app.ConfirmResult(ctx, true); err != nil {
			return errors.Wrap(err, "failed to confirm result")
		}
	}

	// If no path is specified, CCA should work as a regular instance. Skip the
	// result checking part.
	if path == "" {
		return nil
	}

	testing.ContextLog(ctx, "Checking the test image")
	const timeout = 5 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrapf(err, "no matching output file found after %v", timeout)
	}

	testing.ContextLog(ctx, "Removing the test image")
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "failed to delete the test image")
	}
	return nil
}

func checkRecordVideoResult(ctx context.Context, app *cca.App, shouldReview bool, isFolder bool, path string) error {
	// To test the case if the video is generated and put into default video
	// directory after capturing, we need to remove all files inside the default
	// video folder.
	if isFolder {
		if _, err := os.Stat(path); err != nil {
			// It is acceptable that this folder does not exist before running this test. For other
			// errors, we should throw it.
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			dir, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}
			for _, d := range dir {
				os.RemoveAll(fmt.Sprintf("%s/%s", path, d.Name()))
			}
		}
	}

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

	if shouldReview {
		if err := app.WaitForState(ctx, "review-result", true); err != nil {
			return errors.Wrap(err, "does not enter review result state")
		}
		if err := app.CheckConfirmUIExist(ctx, true); err != nil {
			return errors.Wrap(err, "check confirm UI failed")
		}
		if err := app.ConfirmResult(ctx, true); err != nil {
			return errors.Wrap(err, "failed to confirm result")
		}
	}

	// If no path is specified, CCA should work as a regular instance. Skip the
	// result checking part.
	if path == "" {
		return nil
	}

	if isFolder {
		testing.ContextLog(ctx, "Checking the default video folder")
		const timeout = 5 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			files, _ := ioutil.ReadDir(path)
			if len(files) > 0 {
				return nil
			}
			return errors.New("no matching output file found")
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			return errors.Wrapf(err, "no matching output file found after %v", timeout)
		}

		testing.ContextLog(ctx, "Removing files in default video folder")
		dir, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		for _, d := range dir {
			os.RemoveAll(fmt.Sprintf("%s/%s", path, d.Name()))
		}
	} else {
		testing.ContextLog(ctx, "Checking the test video")
		const timeout = 5 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(path); err == nil {
				return nil
			}
			return errors.New("no matching output file found")
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			return errors.Wrapf(err, "no matching output file found after %v", timeout)
		}

		testing.ContextLog(ctx, "Removing the test video")
		if err := os.Remove(path); err != nil {
			return errors.Wrap(err, "failed to delete the test video")
		}
	}
	return nil
}

func checkCCAInstanceExist(ctx context.Context, cr *chrome.Chrome, expected bool) error {
	// Sleeps for a while after capturing and then ensure CCA instance is automatically closed or not.
	testing.ContextLog(ctx, "Checking if CCA is closed")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return err
	}

	if actual, err := cca.IsCCAInstanceExist(ctx, cr); err != nil {
		return err
	} else if actual != expected {
		if expected {
			return errors.New("CCA instance is not automatically closed after capturing")
		}
		return errors.New("CCA instance is automatically closed after capturing")
	}
	return nil
}
