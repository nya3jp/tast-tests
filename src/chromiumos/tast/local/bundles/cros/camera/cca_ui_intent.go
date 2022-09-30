// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
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
	takePhotoAction           = "android.media.action.IMAGE_CAPTURE"
	recordVideoAction         = "android.media.action.VIDEO_CAPTURE"
	launchOnPhotoModeAction   = "android.media.action.STILL_IMAGE_CAMERA"
	launchOnVideoModeAction   = "android.media.action.VIDEO_CAMERA"
	testPhotoURI              = "content://org.chromium.arc.intent_helper.fileprovider/download/test.jpg"
	testVideoURI              = "content://org.chromium.arc.intent_helper.fileprovider/download/test.mp4"
	arcCameraFolderPath       = "data/media/0/DCIM/Camera"
	testAppPkg                = "org.chromium.arc.testapp.cameraintent"
	testAppActivity           = "org.chromium.arc.testapp.cameraintent.MainActivity"
	testAppTextFieldID        = "org.chromium.arc.testapp.cameraintent:id/text"
	testAppSendIntentButtonID = "org.chromium.arc.testapp.cameraintent:id/send_intent"
	resultOK                  = "-1"
	resultCanceled            = "0"
)

var (
	testPhotoPattern      = regexp.MustCompile(`^test\.jpg$`)
	testVideoPattern      = regexp.MustCompile(`^test\.mp4$`)
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies if the camera intents fired from Android apps could be delivered and handled by CCA",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "proprietary_codecs", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      7 * time.Minute,
		Fixture:      "ccaTestBridgeReadyWithArc",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func CCAUIIntent(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	a := s.FixtValue().(cca.FixtureData).ARC
	cr := s.FixtValue().(cca.FixtureData).Chrome
	resetTestBridge := s.FixtValue().(cca.FixtureData).ResetTestBridge

	// In ARCVM, Downloads integration depends on MyFiles mount.
	if err := arc.WaitForARCMyFilesVolumeMountIfARCVMEnabled(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	uiDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer uiDevice.Close(cleanupCtx)

	s.Log("Installing camera intent testing app")
	if err := a.Install(ctx, arc.APKPath("ArcCameraIntentTest.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("ArcIntentHelper did not come up: ", err)
	}

	testing.ContextLog(ctx, "Starting intent behavior tests")

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get Android data dir: ", err)
	}
	arcCameraFolderPathOnChromeOS := filepath.Join(androidDataDir, arcCameraFolderPath)

	userPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user path: ", err)
	}
	downloadsFolder := filepath.Join(userPath, "MyFiles", "Downloads")

	// Ensure that the test can access arcCameraFolderPathOnChromeOS, which is in Android's SDCard partition.
	cleanupFunc, err := arc.MountSDCardPartitionOnHostWithSSHFSIfVirtioBlkDataEnabled(ctx, a, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to make Android's SDCard partition available on host: ", err)
	}
	defer cleanupFunc(cleanupCtx)

	subTestTimeout := 40 * time.Second
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
					Dir:         downloadsFolder,
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
					Dir:         downloadsFolder,
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
		tb := s.FixtValue().(cca.FixtureData).TestBridge()
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.Name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
			defer cancel()
			defer resetTestBridge(cleanupCtx)

			if err := cca.ClearSavedDir(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := checkIntentBehavior(ctx, cr, a, uiDevice, tc.IntentOptions, scripts, outDir, tb); err != nil {
				s.Error("Failed when checking intent behavior: ", err)
			}
		})
		cancel()
	}

	tb := s.FixtValue().(cca.FixtureData).TestBridge()
	subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
	s.Run(subTestCtx, "instances coexistanece test", func(ctx context.Context, s *testing.State) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
		defer cancel()
		defer resetTestBridge(cleanupCtx)

		if err := cca.ClearSavedDir(ctx, cr); err != nil {
			s.Fatal("Failed to clear saved directory: ", err)
		}

		if err := checkInstancesCoexistence(ctx, cr, a, scripts, outDir, tb, uiDevice); err != nil {
			s.Error("Failed for instances coexistence test: ", err)
		}
	})
	cancel()
}

// launchIntent launches CCA intent with different options.
func launchIntent(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, options intentOptions, scripts []string, outDir string, tb *testutil.TestBridge, uiDevice *ui.Device) (*cca.App, error) {
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

		sendIntentButton := uiDevice.Object(ui.ID(testAppSendIntentButtonID))
		if err := sendIntentButton.WaitForExists(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for Send Intent button shown")
		}
		if err := sendIntentButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Send Intent button")
		}
		return nil
	}
	return cca.Init(ctx, cr, scripts, outDir, testutil.AppLauncher{
		LaunchApp:    launchByIntent,
		UseSWAWindow: false,
	}, tb)
}

// ensureCCAClose tries to close CCA if it hasn't been closed yet. Generally,
// this is used as a defer function only to guarantee the app is properly closed
// between sub tests. The closing of the app should be included as a part of the
// test flow so that JS error could be successfully reported even if the test
// passes.
func ensureCCAClose(ctx context.Context, app *cca.App) {
	if err := app.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close CCA: ", err)
	}
}

// ensureTestAppClose tries to close test app if it is running, which is often
// used as a defer function mainly to avoid affecting other sub tests.
func ensureTestAppClose(ctx context.Context, a *arc.ARC) {
	a.Command(ctx, "am", "force-stop", testAppPkg).Run(testexec.DumpLogOnError)
}

// checkIntentBehavior checks basic control flow for handling intent with different options.
func checkIntentBehavior(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, uiDevice *ui.Device, options intentOptions, scripts []string, outDir string, tb *testutil.TestBridge) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	app, err := launchIntent(ctx, cr, a, options, scripts, outDir, tb, uiDevice)
	defer ensureTestAppClose(cleanupCtx, a)
	if err != nil {
		return err
	}
	defer ensureCCAClose(cleanupCtx, app)

	if err := checkUI(ctx, app, options); err != nil {
		return err
	}
	if err := app.CheckMode(ctx, options.Mode); err != nil {
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

	var dir string
	var err error
	if info.Dir == "" {
		dir, err = app.SavedDir(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get CCA default saved path")
		}
	} else {
		dir = info.Dir
	}
	testing.ContextLog(ctx, "Checking capture result")
	if shouldConfirm {
		fileInfo, err := app.WaitForFileSaved(ctx, dir, info.FilePattern, startTime)
		if err != nil {
			return err
		} else if fileInfo.Size() == 0 {
			return errors.New("capture result is empty")
		}
		if err := os.Remove(filepath.Join(dir, fileInfo.Name())); err != nil {
			return errors.Wrap(err, "failed to remove file after confirmation")
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
	// Proactively close the app so that the error happens during closing can
	// be reported.
	if err := app.Close(ctx); err != nil {
		return err
	}
	return nil
}

// checkInstancesCoexistence checks number of CCA windows showing in multiple launch request scenario.
func checkInstancesCoexistence(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, scripts []string, outDir string, tb *testutil.TestBridge, uiDevice *ui.Device) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Launch regular CCA.
	regularApp, err := cca.New(ctx, cr, scripts, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA")
	}
	defer ensureCCAClose(cleanupCtx, regularApp)

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
	}, scripts, outDir, tb, uiDevice)
	defer ensureTestAppClose(cleanupCtx, a)
	if err != nil {
		return errors.Wrap(err, "failed to launch CCA by intent")
	}
	defer ensureCCAClose(cleanupCtx, intentApp)

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
	if err := regularApp.CheckMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to land on video mode when resuming window")
	}
	return nil
}

func checkTestAppResult(ctx context.Context, a *arc.ARC, uiDevice *ui.Device, shouldFinished bool) error {
	textField := uiDevice.Object(ui.ID(testAppTextFieldID))
	if err := textField.WaitForExists(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "the test app UI is not shown within the timeout")
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
