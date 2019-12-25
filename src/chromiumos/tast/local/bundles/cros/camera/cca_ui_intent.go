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
	Action             string
	URI                string
	Mode               cca.Mode
	ShouldReviewResult bool
	ResultInfo         resultInfo
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

const (
	takePhotoAction         = "android.media.action.IMAGE_CAPTURE"
	recordVideoAction       = "android.media.action.VIDEO_CAPTURE"
	launchOnPhotoModeAction = "android.media.action.STILL_IMAGE_CAMERA"
	launchOnVideoModeAction = "android.media.action.VIDEO_CAMERA"
	testPhotoURI            = "content://org.chromium.arc.intent_helper.fileprovider/download/test.jpg"
	testVideoURI            = "content://org.chromium.arc.intent_helper.fileprovider/download/test.mkv"
	defaultArcCameraPath    = "/run/arc/sdcard/write/emulated/0/DCIM/Camera"
)

var (
	testPhotoPattern = regexp.MustCompile(`^test\.jpg$`)
	testVideoPattern = regexp.MustCompile(`^test\.mkv$`)
)

func CCAUIIntent(ctx context.Context, s *testing.State) {
	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	testing.ContextLog(ctx, "Starting intent behavior tests")

	ccaSavedDir, err := cca.GetSavedDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get CCA default saved path")
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             takePhotoAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: true,
		// Currently there is no way to evaluate the returned result of intent which
		// is fired through adb. We might rely on CTS Verifier to test such
		// behavior.
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             takePhotoAction,
		URI:                testPhotoURI,
		Mode:               cca.Photo,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testPhotoPattern,
		},
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             launchOnPhotoModeAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.PhotoPattern,
		},
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             recordVideoAction,
		URI:                "",
		Mode:               cca.Video,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         defaultArcCameraPath,
			FilePattern: cca.VideoPattern,
		},
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             recordVideoAction,
		URI:                testVideoURI,
		Mode:               cca.Video,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testVideoPattern,
		},
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, intentOptions{
		Action:             launchOnVideoModeAction,
		URI:                "",
		Mode:               cca.Video,
		ShouldReviewResult: false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.VideoPattern,
		},
	}); err != nil {
		s.Error("Failed for intent behavior test: ", err)
	}

	if err := checkInstancesCoexistence(ctx, s, cr, a); err != nil {
		s.Error("Failed for instance coexistence test: ", err)
	}

	// TODO(wtlee): Add intent cancelation tests
}

func launchIntent(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, options intentOptions) (*cca.App, error) {
	return cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), func(tconn *chrome.Conn) error {
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
}

func checkIntentBehavior(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, options intentOptions) error {
	app, err := launchIntent(ctx, s, cr, a, options)
	if err != nil {
		return err
	}
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
	// For intents which expects to receive result, CCA should show review UI
	// after the capture is done.
	if err := checkCaptureResult(ctx, app, options.Mode, options.ShouldReviewResult, options.ResultInfo); err != nil {
		return err
	}
	// For intents which expects to receive result, CCA should auto close after
	// the capture is done.
	if err := checkAutoCloseBehavior(ctx, cr, options.ShouldReviewResult); err != nil {
		return err
	}
	return nil
}

func checkLandingMode(ctx context.Context, app *cca.App, mode cca.Mode) error {
	if result, err := app.GetState(ctx, string(mode)); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.Errorf("CCA does not land on the expected mode: %s", mode)
	}
	return nil
}

func checkCaptureResult(ctx context.Context, app *cca.App, mode cca.Mode, shouldReviewResult bool, info resultInfo) error {
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

	if shouldReviewResult {
		if err := app.ConfirmResult(ctx, true, mode); err != nil {
			return errors.Wrap(err, "failed to confirm result")
		}
	} else {
		if err := app.WaitForState(ctx, "taking", false); err != nil {
			return errors.Wrap(err, "shutter is not ended")
		}
	}

	// If there is no result information, skip the result checking part.
	if info == (resultInfo{}) {
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
	// Sleeps for a while after capturing and then ensure CCA instance is
	// automatically closed or not.
	testing.ContextLog(ctx, "Checking auto close behavior")
	if shouldClose {
		const timeout = 1 * time.Second
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if isExist, err := cca.InstanceExists(ctx, cr); err != nil {
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
		if isExist, err := cca.InstanceExists(ctx, cr); err != nil {
			return err
		} else if !isExist {
			return errors.New("CCA instance is automatically closed after capturing")
		}
	}
	return nil
}

func checkInstancesCoexistence(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC) error {
	// Launch regular CCA.
	regularApp, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA")
	}
	defer regularApp.Close(ctx)

	// Launch camera intent.
	intentApp, err := launchIntent(ctx, s, cr, a, intentOptions{
		Action:             takePhotoAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: true,
	})
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA by intent")
	}
	defer intentApp.Close(ctx)

	// Check if the regular CCA is suspeneded.
	if err := regularApp.WaitForState(ctx, "suspend", true); err != nil {
		return errors.Wrap(err, "regular app instance does not suspend after launching intent")
	}

	// Close intent CCA instance.
	if err := intentApp.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close intent instance")
	}

	// Check if the regular CCA is automatically resumed.
	if err := regularApp.WaitForState(ctx, "suspend", false); err != nil {
		return errors.Wrap(err, "regular app instance does not resume after closing intent instance")
	}

	return nil
}
