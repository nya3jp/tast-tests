// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
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

const (
	takePhotoAction         = "android.media.action.IMAGE_CAPTURE"
	recordVideoAction       = "android.media.action.VIDEO_CAPTURE"
	launchOnPhotoModeAction = "android.media.action.STILL_IMAGE_CAMERA"
	launchOnVideoModeAction = "android.media.action.VIDEO_CAMERA"
	testPhotoURI            = "content://org.chromium.arc.intent_helper.fileprovider/download/test.jpg"
	// TODO(https://crbug.com/1058325): Change all mkv to mp4 once the migration is done.
	// The content of test.mkv might be mp4 during the mkv to mp4 migration period.
	testVideoURI        = "content://org.chromium.arc.intent_helper.fileprovider/download/test.mkv"
	arcCameraFolderPath = "data/media/0/DCIM/Camera"
	testAppAPK          = "ArcCameraIntentTest.apk"
	testAppPkg          = "org.chromium.arc.testapp.cameraintent"
	testAppActivity     = "org.chromium.arc.testapp.cameraintent.MainActivity"
	testAppTextFieldID  = "org.chromium.arc.testapp.cameraintent:id/text"
	resultOK            = "-1"
	resultCanceled      = "0"
)

var (
	testPhotoPattern      = regexp.MustCompile(`^test\.jpg$`)
	testVideoPattern      = regexp.MustCompile(`^test\.(mkv|mp4)$`)
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIIntent,
		Desc:         "Verifies if the camera intents fired from Android apps could be delivered and handled by CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "proprietary_codecs", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js", "ArcCameraIntentTest.apk"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.Booted(),
		}},
	})
}

func CCAUIIntent(ctx context.Context, s *testing.State) {
	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	uiDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer uiDevice.Close(ctx)

	s.Log("Installing camera intent testing app")
	if err := a.Install(ctx, s.DataPath(testAppAPK)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	testing.ContextLog(ctx, "Starting intent behavior tests")

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	androidDataDir, err := arc.AndroidDataDir(cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get Android data dir: ", err)
	}
	arcCameraFolderPathOnChromeOS := filepath.Join(androidDataDir, arcCameraFolderPath)

	subTestTimeout := 20 * time.Second
	for _, tc := range []struct {
		Name          string
		IntentOptions intentOptions
	}{
		{
			Name: "take photo (no extra)",
			IntentOptions: intentOptions{
				Action:       takePhotoAction,
				URI:          "",
				Mode:         cca.Photo,
				TestBehavior: captureConfirmAndDone,
			},
		}, {
			Name: "take photo (has extra)",
			IntentOptions: intentOptions{
				Action:       takePhotoAction,
				URI:          testPhotoURI,
				Mode:         cca.Photo,
				TestBehavior: captureConfirmAndDone,
				ResultInfo: resultInfo{
					FilePattern: testPhotoPattern,
				},
			},
		}, {
			Name: "launch camera on photo mode",
			IntentOptions: intentOptions{
				Action:       launchOnPhotoModeAction,
				URI:          "",
				Mode:         cca.Photo,
				TestBehavior: captureAndAlive,
				ResultInfo: resultInfo{
					FilePattern: cca.PhotoPattern,
				},
			},
		}, {
			Name: "launch camera on video mode",
			IntentOptions: intentOptions{
				Action:       launchOnVideoModeAction,
				URI:          "",
				Mode:         cca.Video,
				TestBehavior: captureAndAlive,
				ResultInfo: resultInfo{
					FilePattern: cca.VideoPattern,
				},
			},
		}, {
			Name: "record video (no extras)",
			IntentOptions: intentOptions{
				Action:       recordVideoAction,
				URI:          "",
				Mode:         cca.Video,
				TestBehavior: captureConfirmAndDone,
				ResultInfo: resultInfo{
					Dir:         arcCameraFolderPathOnChromeOS,
					FilePattern: cca.VideoPattern,
				},
			},
		}, {
			Name: "record video (has extras)",
			IntentOptions: intentOptions{
				Action:       recordVideoAction,
				URI:          testVideoURI,
				Mode:         cca.Video,
				TestBehavior: captureConfirmAndDone,
				ResultInfo: resultInfo{
					FilePattern: testVideoPattern,
				},
			},
		}, {
			Name: "close app",
			IntentOptions: intentOptions{
				Action:       takePhotoAction,
				URI:          "",
				Mode:         cca.Photo,
				TestBehavior: closeApp,
			},
		}, {
			Name: "cancel when review",
			IntentOptions: intentOptions{
				Action:       takePhotoAction,
				URI:          "",
				Mode:         cca.Photo,
				TestBehavior: captureCancelAndAlive,
			},
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.Name, func(ctx context.Context, s *testing.State) {
			if err := cca.ClearSavedDirs(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := checkIntentBehavior(ctx, cr, a, uiDevice, tc.IntentOptions, scripts, outDir, tb); err != nil {
				s.Error("Failed when checking intent behavior: ", err)
			}
		})
		cancel()
	}

	subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
	s.Run(subTestCtx, "instances coexistanece test", func(ctx context.Context, s *testing.State) {
		if err := cca.ClearSavedDirs(ctx, cr); err != nil {
			s.Fatal("Failed to clear saved directory: ", err)
		}

		if err := checkInstancesCoexistence(ctx, cr, a, scripts, outDir, tb); err != nil {
			s.Error("Failed for instances coexistence test: ", err)
		}
	})
	cancel()
}

// launchIntent launches CCA intent with different options.
func launchIntent(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, options intentOptions, scripts []string, outDir string, tb *testutil.TestBridge) (*cca.App, error) {
	launchByIntent := func(ctx context.Context, tconn *chrome.TestConn) error {
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
	}
	return cca.Init(ctx, cr, scripts, outDir, testutil.AppLauncher{
		LaunchApp:    launchByIntent,
		UseSWAWindow: false,
	}, tb)
}

func closeCCAAndTestApp(ctx context.Context, a *arc.ARC, app *cca.App, outDir string) error {
	err := app.Close(ctx)
	// TODO(crbug.com/980846): For intents, since it will close itself once the
	// intent is handled, it is very likely that app cannot be closed properly
	// on Tast side due to the connection is closing. As a result, only log the
	// error as a temporary workaround.
	if err != nil {
		testing.ContextLog(ctx, "Failed to close CCA: ", err)
		err = nil
	}
	a.Command(ctx, "am", "force-stop", testAppPkg).Run(testexec.DumpLogOnError)
	return err
}

// checkIntentBehavior checks basic control flow for handling intent with different options.
func checkIntentBehavior(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, uiDevice *ui.Device, options intentOptions, scripts []string, outDir string, tb *testutil.TestBridge) (retErr error) {
	app, err := launchIntent(ctx, cr, a, options, scripts, outDir, tb)
	if err != nil {
		return err
	}
	defer func(ctx context.Context) {
		if err := closeCCAAndTestApp(ctx, a, app, outDir); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to close CCA and test app: ", err)
			} else {
				retErr = err
			}
		}
	}(ctx)

	if err := checkUI(ctx, app, options); err != nil {
		return err
	}
	if err := checkLandingMode(ctx, app, options.Mode); err != nil {
		return err
	}

	if options.TestBehavior.ShouldCloseDirectly {
		if err := app.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close app")
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
		if err := checkAutoCloseBehavior(ctx, cr, app, shouldAppAutoClose); err != nil {
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

	var dirs []string
	var err error
	if info.Dir == "" {
		dirs, err = app.SavedDirs(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get CCA default saved path")
		}
	} else {
		dirs = []string{info.Dir}
	}
	testing.ContextLog(ctx, "Checking capture result")
	if shouldConfirm {
		if fileInfo, err := app.WaitForFileSaved(ctx, dirs, info.FilePattern, startTime); err != nil {
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
func checkAutoCloseBehavior(ctx context.Context, cr *chrome.Chrome, app *cca.App, shouldClose bool) error {
	// Sleeps for a while after capturing and then ensure CCA instance is
	// automatically closed or not.
	testing.ContextLog(ctx, "Checking auto close behavior")
	result := (func() error {
		if shouldClose {
			const timeout = 3 * time.Second
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if isClosing, err := app.ClosingItself(ctx); err != nil {
					return testing.PollBreak(err)
				} else if isClosing {
					return nil
				}
				return errors.New("CCA instance is not automatically closed after capturing")
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				return err
			}
		} else {
			if err := testing.Sleep(ctx, 3*time.Second); err != nil {
				return err
			}
			if isClosing, err := app.ClosingItself(ctx); err != nil {
				return err
			} else if isClosing {
				return errors.New("CCA instance is automatically closed after capturing")
			}
		}
		return nil
	})()

	if err := app.Close(ctx); err != nil {
		if result == nil {
			return err
		}
	}
	return result
}

// checkInstancesCoexistence checks number of CCA windows showing in multiple launch request scenario.
func checkInstancesCoexistence(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, scripts []string, outDir string, tb *testutil.TestBridge) (retErr error) {
	// Launch regular CCA.
	regularApp, err := cca.New(ctx, cr, scripts, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA")
	}
	defer func(ctx context.Context) {
		if err := closeCCAAndTestApp(ctx, a, regularApp, outDir); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to close CCA and test app: ", err)
			} else {
				retErr = err
			}
		}
	}(ctx)

	// Switch to video mode to check if the mode remains the same after resuming.
	if err := regularApp.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch mode")
	}

	// Launch camera intent.
	intentApp, err := launchIntent(ctx, cr, a, intentOptions{
		Action:       takePhotoAction,
		URI:          "",
		Mode:         cca.Photo,
		TestBehavior: captureConfirmAndDone,
	}, scripts, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA by intent")
	}

	// Check if the regular CCA is suspeneded.
	if err := regularApp.WaitForState(ctx, "suspend", true); err != nil {
		return errors.Wrap(err, "regular app instance does not suspend after launching intent")
	}

	// Close intent CCA instance.
	if err := intentApp.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close intent instance")
	}

	// We don't automatically show the original window back.
	if err := regularApp.Focus(ctx); err != nil {
		return errors.Wrap(err, "failed to focus the original window")
	}
	// Check if the regular CCA is automatically resumed.
	if err := regularApp.WaitForState(ctx, "suspend", false); err != nil {
		return errors.Wrap(err, "regular app instance does not resume after closing intent instance")
	}

	// Check if the regular CCA still lands on video mode.
	if err := checkLandingMode(ctx, regularApp, cca.Video); err != nil {
		return errors.Wrap(err, "failed to land on video mode when resuming window")
	}
	return nil
}

func checkTestAppResult(ctx context.Context, a *arc.ARC, uiDevice *ui.Device, shouldFinished bool) error {
	textField := uiDevice.Object(ui.ID(testAppTextFieldID))
	if err := textField.WaitForExists(ctx, 5*time.Second); err != nil {
		// TODO(b/148995660): These lines are added since the test app sometimes will be minimized after
		// launching CCA. Remove these lines once the issue is resolved.
		args := []string{"start", "--activity-brought-to-front", "-n", fmt.Sprintf("%s/%s", testAppPkg, testAppActivity)}
		if _, err := a.Command(ctx, "am", args...).Output(testexec.DumpLogOnError); err != nil {
			return err
		}
		if err := textField.WaitForExists(ctx, 5*time.Second); err != nil {
			return err
		}
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
