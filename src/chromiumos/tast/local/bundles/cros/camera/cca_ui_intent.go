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

type intentOptions struct {
	Action            string
	URI               string
	Mode              cca.Mode
	ShouldReview      bool
	ShouldCheckResult bool
	ShouldAutoClose   bool
	ResultInfo        resultInfo
}

type resultInfo struct {
	Dir         string
	FilePattern *regexp.Regexp
}

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

	checkIntent := func(options intentOptions) error {
		app, err := cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, func(tconn *chrome.Conn) error {
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			testing.ContextLogf(ctx, "Testing action: %s", options.Action)

			args := []string{"start", "-a", options.Action}
			if options.URI != "" {
				args = append(args, "--eu", "output", options.URI)
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
		if err := checkLandingMode(ctx, app, options.Mode); err != nil {
			return err
		}
		if err := checkCaptureResult(ctx, app, options.Mode, options.ShouldReview, options.ShouldCheckResult, options.ResultInfo); err != nil {
			return err
		}
		if err := checkAutoCloseBehavior(ctx, cr, options.ShouldAutoClose); err != nil {
			return err
		}
		return nil
	}

	testing.ContextLog(ctx, "Starting intent behavior tests")

	ccaSavedDir, err := cca.GetSavedDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get CCA default saved path")
	}

	checkIntent(intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		ShouldReview: true,
		// Currently there is no way to evaluate the returned result of intent which is fired through
		// adb. We might rely on CTS Verifier to test such behavior.
		ShouldCheckResult: false,
		ShouldAutoClose:   true,
	})

	checkIntent(intentOptions{
		Action:            takePhotoAction,
		URI:               testPhotoURI,
		Mode:              cca.Photo,
		ShouldReview:      true,
		ShouldCheckResult: true,
		ShouldAutoClose:   true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testPhotoPattern,
		},
	})

	checkIntent(intentOptions{
		Action:            launchOnPhotoModeAction,
		URI:               "",
		Mode:              cca.Photo,
		ShouldReview:      false,
		ShouldCheckResult: true,
		ShouldAutoClose:   false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.PhotoPattern,
		},
	})

	checkIntent(intentOptions{
		Action:            recordVideoAction,
		URI:               "",
		Mode:              cca.Video,
		ShouldReview:      true,
		ShouldCheckResult: true,
		ShouldAutoClose:   true,
		ResultInfo: resultInfo{
			Dir:         defaultArcCameraPath,
			FilePattern: cca.VideoPattern,
		},
	})

	checkIntent(intentOptions{
		Action:            recordVideoAction,
		URI:               testVideoURI,
		Mode:              cca.Video,
		ShouldReview:      true,
		ShouldCheckResult: true,
		ShouldAutoClose:   true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testVideoPattern,
		},
	})

	checkIntent(intentOptions{
		Action:            launchOnVideoModeAction,
		URI:               "",
		Mode:              cca.Video,
		ShouldReview:      false,
		ShouldCheckResult: true,
		ShouldAutoClose:   false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.VideoPattern,
		},
	})

	// TODO(wtlee): Add intent cancelation tests

	// TODO(wtlee): Add intent instances coexistance tests
}

func checkLandingMode(ctx context.Context, app *cca.App, mode cca.Mode) error {
	if result, err := app.GetState(ctx, string(mode)); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.New("CCA does not land on correct mode")
	}
	return nil
}

func checkCaptureResult(ctx context.Context, app *cca.App, mode cca.Mode, shouldReview bool, shouldCheckResult bool, info resultInfo) error {
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

		// CCA will create a tempoary file when starting recording. To get the real
		// result file, we should pass the time which is later than the creation
		// time of the temporary file.
		startTime = time.Now()

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

	if !shouldCheckResult {
		return nil
	}

	testing.ContextLog(ctx, "Checking capture result")
	if fileInfo, err := app.WaitForFileSaved(ctx, info.Dir, info.FilePattern, startTime); err != nil {
		return err
	} else if fileInfo.Size() == 0 {
		return errors.New("capture result is empty")
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
