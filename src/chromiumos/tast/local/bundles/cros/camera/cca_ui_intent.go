// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
	Action             string
	URI                string
	Mode               cca.Mode
	ShouldReviewResult bool
	ResultInfo         resultInfo
	LaunchMethod       launchMethod
	ShouldCancel       bool
}

type launchMethod string

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
	testPhotoURI            = "file:///sdcard/Download/test.png"
	testVideoURI            = "file:///sdcard/Download/test.mkv"
	defaultArcCameraPath    = "/run/arc/sdcard/write/emulated/0/DCIM/Camera"
	testAppAPK              = "ArcCameraIntentTest.apk"
	testAppPkg              = "org.chromium.arc.testapp.cameraintent"
	testAppActivity         = "org.chromium.arc.testapp.cameraintent.MainActivity"
	testAppTextFieldID      = "org.chromium.arc.testapp.cameraintent:id/text"
	adb                     = "adb"
	testApp                 = "testApp"
)

var (
	testPhotoPattern = regexp.MustCompile(`^test\.png$`)
	testVideoPattern = regexp.MustCompile(`^test\.mkv$`)
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
		Action:             takePhotoAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: true,
		// Currently there is no way to evaluate the returned result of intent which
		// is fired through adb. We evaluate the capture result of such intent in
		// camera intent testing app.
		LaunchMethod: testApp,
	}); err != nil {
		s.Fatal("Failed for intent behavior test via test app: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             takePhotoAction,
		URI:                testPhotoURI,
		Mode:               cca.Photo,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testPhotoPattern,
		},
		LaunchMethod: adb,
	}); err != nil {
		s.Fatal("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             launchOnPhotoModeAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.PhotoPattern,
		},
		LaunchMethod: adb,
	}); err != nil {
		s.Fatal("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             recordVideoAction,
		URI:                "",
		Mode:               cca.Video,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         defaultArcCameraPath,
			FilePattern: cca.VideoPattern,
		},
		LaunchMethod: adb,
	}); err != nil {
		s.Fatal("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             recordVideoAction,
		URI:                testVideoURI,
		Mode:               cca.Video,
		ShouldReviewResult: true,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: testVideoPattern,
		},
		LaunchMethod: adb,
	}); err != nil {
		s.Fatal("Failed for intent behavior test: ", err)
	}

	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             launchOnVideoModeAction,
		URI:                "",
		Mode:               cca.Video,
		ShouldReviewResult: false,
		ResultInfo: resultInfo{
			Dir:         ccaSavedDir,
			FilePattern: cca.VideoPattern,
		},
		LaunchMethod: adb,
	}); err != nil {
		s.Fatal("Failed for intent behavior test: ", err)
	}

	if err := checkInstancesCoexistance(ctx, s, cr, a); err != nil {
		s.Fatal("Failed for instance coexistance test: ", err)
	}

	// Test the cancelation behavior.
	if err := checkIntentBehavior(ctx, s, cr, a, uiDevice, intentOptions{
		Action:             takePhotoAction,
		URI:                "",
		Mode:               cca.Photo,
		ShouldReviewResult: true,
		LaunchMethod:       testApp,
		ShouldCancel:       true,
	}); err != nil {
		s.Fatal("Failed for intent behavior test via test app: ", err)
	}
}

func launchIntent(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, options intentOptions) (*cca.App, error) {
	shouldDownScale := options.Action == takePhotoAction && options.ResultInfo == (resultInfo{})
	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", cca.ID)
	btoi := func(val bool) int {
		if val {
			return 1
		}
		return 0
	}
	var modeStr string
	if options.Mode == cca.Photo {
		modeStr = "photo"
	} else if options.Mode == cca.Video {
		modeStr = "video"
	} else {
		return nil, errors.Errorf("unrecognized mode: %s", options.Mode)
	}

	return cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, func(tconn *chrome.Conn) error {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		var args []string
		if options.LaunchMethod == adb {
			testing.ContextLogf(ctx, "Testing action: %s", options.Action)

			args = []string{"start", "-a", options.Action}
			if options.URI != "" {
				args = append(args, "--eu", "output", options.URI)
			}
		} else if options.LaunchMethod == testApp {
			args = []string{"start", fmt.Sprintf("%s/%s", testAppPkg, testAppActivity)}
		} else {
			return errors.Errorf("unknown launch by value: %s", options.LaunchMethod)
		}

		output, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		testing.ContextLog(ctx, string(output))
		return nil
	}, func(t *chrome.Target) bool {
		return strings.HasPrefix(t.URL, ccaURL) &&
			strings.Contains(t.URL, fmt.Sprintf("mode=%s", modeStr)) &&
			strings.Contains(t.URL, fmt.Sprintf("shouldHandleResult=%d", btoi(options.ShouldReviewResult))) &&
			strings.Contains(t.URL, fmt.Sprintf("shouldDownScale=%d", btoi(shouldDownScale)))
	})
}

func cleanup(ctx context.Context, a *arc.ARC, launchMethod launchMethod) {
	if launchMethod == testApp {
		args := []string{"force-stop", testAppPkg}
		a.Command(ctx, "am", args...).Run(testexec.DumpLogOnError)
	}
}

func checkIntentBehavior(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, uiDevice *ui.Device, options intentOptions) error {
	app, err := launchIntent(ctx, s, cr, a, options)
	if err != nil {
		return err
	}
	defer app.Close(ctx)
	defer cleanup(ctx, a, options.LaunchMethod)

	if err != nil {
		return err
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		return err
	}
	if err := checkLandingMode(ctx, app, options.Mode); err != nil {
		return err
	}
	if options.ShouldCancel {
		if err := app.Close(ctx); err != nil {
			return err
		}
	} else {
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
	}

	if options.LaunchMethod == testApp {
		if err := checkTestAppResult(ctx, uiDevice, options.ShouldCancel); err != nil {
			return err
		}
	}
	return nil
}

func checkLandingMode(ctx context.Context, app *cca.App, mode cca.Mode) error {
	if result, err := app.GetState(ctx, string(mode)); err != nil {
		return errors.Wrap(err, "failed to check state")
	} else if !result {
		return errors.New("CCA does not land on correct mode")
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

func checkInstancesCoexistance(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC) error {
	// Launch regular CCA.
	regularApp, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
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
		LaunchMethod:       adb,
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

func checkTestAppResult(ctx context.Context, uiDevice *ui.Device, shouldCancel bool) error {
	textField := uiDevice.Object(ui.ID(testAppTextFieldID))
	if err := textField.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}

	text, err := textField.GetText(ctx)
	if err != nil {
		return err
	}

	if shouldCancel {
		if text != "Canceled" {
			return errors.New("the end state of the testing app is not canceled")
		}
	} else {
		if text != "Ok" {
			return errors.New("the end state of the testing app is not Ok")
		}
	}
	return nil
}
