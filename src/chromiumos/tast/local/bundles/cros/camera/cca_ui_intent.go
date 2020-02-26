// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type intentOptions struct {
	Action       string
	URI          string
	Mode         cca.Mode
	TestBehavior testBehavior
	ResultInfo   resultInfo
}

type testBehavior struct {
	ShouldReview bool
	// ShouldConfirmAfterCapture indicates if it should click confirm button after capturing. If false, it should click the cancel button.
	ShouldConfirmAfterCapture bool
	ShouldCloseDirectly       bool
	ShouldShowResultInApp     bool
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
		Data:         []string{"cca_ui.js", "ArcCameraIntentTest.apk"},
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
	testAppAPK              = "ArcCameraIntentTest.apk"
	testAppPkg              = "org.chromium.arc.testapp.cameraintent"
	testAppActivity         = "org.chromium.arc.testapp.cameraintent.MainActivity"
	testAppTextFieldID      = "org.chromium.arc.testapp.cameraintent:id/text"
	resultOK                = "-1"
	resultCanceled          = "0"
)

var (
	testPhotoPattern      = regexp.MustCompile(`^test\.jpg$`)
	testVideoPattern      = regexp.MustCompile(`^test\.mkv$`)
	captureConfirmAndDone = testBehavior{
		ShouldReview:              true,
		ShouldConfirmAfterCapture: true,
		ShouldShowResultInApp:     true,
	}
	captureCancelAndAlive = testBehavior{
		ShouldReview:              true,
		ShouldConfirmAfterCapture: false,
		ShouldShowResultInApp:     false,
	}
	captureAndAlive = testBehavior{
		ShouldReview:          false,
		ShouldShowResultInApp: false,
	}
	closeApp = testBehavior{
		ShouldReview:          true,
		ShouldCloseDirectly:   true,
		ShouldShowResultInApp: true,
	}
)

func CCAUIIntent(ctx context.Context, s *testing.State) {
	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	uiDevice, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}

	s.Log("Installing camera intent testing app")
	if err := a.Install(ctx, s.DataPath(testAppAPK)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	testing.ContextLog(ctx, "Starting intent behavior tests")

	ccaSavedDir, err := cca.GetSavedDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get CCA default saved path")
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: captureConfirmAndDone,
	}); err != nil {
		s.Error("Failed for take photo (no extra) behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       takePhotoAction,
		URI:          testPhotoURI,
		Mode:         cca.Photo,
		TestBehavior: captureConfirmAndDone,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testPhotoPattern,
		},
	}); err != nil {
		s.Error("Failed for take photo (has extra) behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       launchOnPhotoModeAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: captureAndAlive,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.PhotoPattern,
		},
	}); err != nil {
		s.Error("Failed for launch camera on photo mode behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       recordVideoAction,
		URI:          "",
		Mode:         cca.Video,
		TestBehavior: captureConfirmAndDone,
		ResultInfo: resultInfo{
			Dir:         defaultArcCameraPath,
			FilePattern: cca.VideoPattern,
		},
	}); err != nil {
		s.Error("Failed for record video (no extras) behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       recordVideoAction,
		URI:          testVideoURI,
		Mode:         cca.Video,
		TestBehavior: captureConfirmAndDone,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testVideoPattern,
		},
	}); err != nil {
		s.Error("Failed for record video (has extras) behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       launchOnVideoModeAction,
		URI:          "",
		Mode:         cca.Video,
		TestBehavior: captureAndAlive,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.VideoPattern,
		},
	}); err != nil {
		s.Error("Failed for launch camera on video mode behavior test: ", err)
	}

	if err := checkInstancesCoexistence(ctx, s, cr, a); err != nil {
		s.Error("Failed for instance coexistence test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: closeApp,
	}); err != nil {
		s.Error("Failed for close app behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: captureCancelAndAlive,
	}); err != nil {
		s.Error("Failed for cancel when review behavior test: ", err)
	}

	// TODO(b/139650048): We may want more complicated test. For example, capture,
	// cancel at the first time, capture again and confirm the result.
}

// launchIntent launches CCA intent with different options.
func launchIntent(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, options intentOptions) (*cca.App, error) {
	return cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), func(tconn *chrome.TestConn) error {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		args := []string{"start", "-n", fmt.Sprintf("%s/%s", testAppPkg, testAppActivity), "-e", "action", options.Action}
		if options.URI != "" {
			args = append(args, "--eu", "uri", options.URI)
		}

		testing.ContextLogf(ctx, "Testing action: %s", options.Action)
		output, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		testing.ContextLog(ctx, string(output))
		return nil
	})
}

func cleanup(ctx context.Context, a *arc.ARC) {
	a.Command(ctx, "am", "force-stop", testAppPkg).Run(testexec.DumpLogOnError)
}

// checkIntentBehavior checks basic control flow for handling intent with different options.
func checkIntentBehavior(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, uiDevice *ui.Device, options intentOptions) error {
	app, err := launchIntent(ctx, s, cr, a, options)
	if err != nil {
		return err
	}
	defer app.Close(ctx)
	defer cleanup(ctx, a)

	if err := checkUI(ctx, app, options); err != nil {
		return err
	}
	if err := checkLandingMode(ctx, app, options.Mode); err != nil {
		return err
	}

	if options.TestBehavior.ShouldCloseDirectly {
		if err := app.Close(ctx); err != nil {
			return err
		}
	} else {
		startTime, err := capture(ctx, app, options.Mode)
		if err != nil {
			return err
		}

		if options.TestBehavior.ShouldReview {
			if err := checkCaptureResult(ctx, app, options.Mode, startTime, options.TestBehavior.ShouldConfirmAfterCapture, options.ResultInfo); err != nil {
				return err
			}
		} else {
			if err := app.WaitForState(ctx, "taking", false); err != nil {
				return errors.Wrap(err, "shutter is not ended")
			}
		}

		shouldAppAutoClose := options.TestBehavior.ShouldReview && options.TestBehavior.ShouldConfirmAfterCapture
		if err := checkAutoCloseBehavior(ctx, cr, shouldAppAutoClose); err != nil {
			return err
		}
	}

	if options.TestBehavior.ShouldShowResultInApp {
		shouldFinished := options.TestBehavior.ShouldReview && options.TestBehavior.ShouldConfirmAfterCapture
		if err := checkTestAppResult(ctx, a, uiDevice, shouldFinished); err != nil {
			return err
		}
	}
	return nil
}

// checkLandingMode checks whether CCA window lands in correct capture mode.
func checkLandingMode(ctx context.Context, app *cca.App, mode cca.Mode) error {
	if result, err := app.GetState(ctx, string(mode)); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.Errorf("CCA does not land on the expected mode: %s", mode)
	}
	return nil
}

// checkUI checks states of UI components in CCA window handling intent with different options.
func checkUI(ctx context.Context, app *cca.App, options intentOptions) error {
	for _, tst := range []struct {
		ui       cca.UIComponent
		expected bool
	}{
		{cca.ModeSelector, !options.TestBehavior.ShouldReview},
		{cca.SettingsButton, !options.TestBehavior.ShouldReview},
	} {
		if err := app.CheckVisible(ctx, tst.ui, tst.expected); err != nil {
			return err
		}
	}
	return nil
}

func capture(ctx context.Context, app *cca.App, mode cca.Mode) (time.Time, error) {
	startTime := time.Now()
	if mode == cca.Photo {
		testing.ContextLog(ctx, "Taking a photo")
		if err := app.ClickShutter(ctx); err != nil {
			return startTime, errors.Wrap(err, "failed to click shutter button")
		}
	} else if mode == cca.Video {
		testing.ContextLog(ctx, "Recording a video")
		if err := app.ClickShutter(ctx); err != nil {
			return startTime, errors.Wrap(err, "failed to click shutter button")
		}
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return startTime, err
		}

		// CCA will create a tempoary file when starting recording. To get the real
		// result file, we should pass the time which is later than the creation
		// time of the temporary file.
		startTime = time.Now()

		testing.ContextLog(ctx, "Stopping a video")
		if err := app.ClickShutter(ctx); err != nil {
			return startTime, errors.Wrap(err, "failed to click shutter button")
		}
	} else {
		return startTime, errors.Errorf("unrecognized mode: %s", mode)
	}
	return startTime, nil
}

func checkCaptureResult(ctx context.Context, app *cca.App, mode cca.Mode, startTime time.Time, shouldConfirm bool, info resultInfo) error {
	if err := app.ConfirmResult(ctx, shouldConfirm, mode); err != nil {
		return errors.Wrap(err, "failed to confirm result")
	}

	// If there is no result information, skip the result checking part.
	if info == (resultInfo{}) {
		return nil
	}

	testing.ContextLog(ctx, "Checking capture result")
	if shouldConfirm {
		if fileInfo, err := app.WaitForFileSaved(ctx, info.Dir, info.FilePattern, startTime); err != nil {
			return err
		} else if fileInfo.Size() == 0 {
			return errors.New("capture result is empty")
		}
	} else {
		// TODO(b/139650048): We should verify if the temporary file is deleted
		// after clicking cancel button.
	}
	return nil
}

// checkAutoCloseBehavior checks closing state of CCA window in intent control flow.
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

// checkInstancesCoexistence checks number of CCA windows showing in multiple launch request scenario.
func checkInstancesCoexistence(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC) error {
	// Launch regular CCA.
	regularApp, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA")
	}
	defer regularApp.Close(ctx)

	// Launch camera intent.
	intentApp, err := launchIntent(ctx, s, cr, a, intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: captureConfirmAndDone,
	})
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA by intent")
	}
	defer intentApp.Close(ctx)
	defer cleanup(ctx, a)

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

func checkTestAppResult(ctx context.Context, a *arc.ARC, uiDevice *ui.Device, shouldFinished bool) error {
	// TODO(b/148995660): These lines are added since the test app sometimes will be minimized after
	// launching CCA. Remove these lines once the issue is resolved.
	args := []string{"start", "--activity-single-top", "-n", fmt.Sprintf("%s/%s", testAppPkg, testAppActivity)}
	if _, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError); err != nil {
		return err
	}

	textField := uiDevice.Object(ui.ID(testAppTextFieldID))
	if err := textField.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}

	text, err := textField.GetText(ctx)
	if err != nil {
		return err
	}

	var expected string
	if shouldFinished {
		expected = resultOK
	} else {
		expected = resultCanceled
	}
	if text != expected {
		return errors.Errorf("unexpected end state of the testing app: got: %q, want: %q", text, expected)
	}
	return nil
}
