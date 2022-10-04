// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/camera/chart"
	dutcontrol "chromiumos/tast/common/camera/dut"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	ccaSetUpTimeout        = 25 * time.Second
	ccaTearDownTimeout     = 5 * time.Second
	testBridgeSetUpTimeout = 20 * time.Second
	setUpTimeout           = chrome.LoginTimeout + testBridgeSetUpTimeout
	tearDownTimeout        = chrome.ResetTimeout
)

type feature string

const (
	multiPageDocScan feature = "CameraAppMultiPageDocScan"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedInCameraBox",
		Desc:            "Launched CCA in a Camera Box",
		Contacts:        []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{launchCCAInCameraBox: true, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunched",
		Desc:            "Launched CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedGuest",
		Desc:            "Launched CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{guestMode: true, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithFakeCamera",
		Desc:            "Launched CCA with fake camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReady",
		Desc:            "Set up test bridge for CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyLacros",
		Desc:            "Set up test bridge for CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{lacros: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "ccaTestBridgeReadyWithFakeCamera",
		Desc: `Set up test bridge for CCA with fake camera. Any tests using this
		       fixture should switch the camera scene before opening camera`,
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, fakeScene: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "ccaTestBridgeReadyWithFakeCameraLacros",
		Desc: `Set up test bridge for CCA with fake camera. Any tests using this
		       fixture should switch the camera scene before opening camera`,
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, fakeScene: true, lacros: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyBypassPermissionClamshell",
		Desc:            "Set up test bridge for CCA with bypassPermission on clamshell mode on",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{bypassPermission: true, forceClamshell: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyWithArc",
		Desc:            "Set up test bridge for CCA with ARC enabled",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{arcBooted: true},
		SetUpTimeout:    setUpTimeout + arc.BootTimeout + ui.StartTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "ccaTestBridgeReadyForMultiPageDocScan",
		Desc:     "Set up test bridge for CCA and chrome for testing multi-page document scanning",
		Contacts: []string{"chuhsuan@chromium.org"},
		Data:     []string{"cca_ui.js"},
		Impl: &fixture{
			fakeCamera: true,
			fakeScene:  true,
			features:   []feature{multiPageDocScan},
		},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyWithAutoFramingForceEnabled",
		Desc:            "Set up test bridge for CCA with Auto Framing force enabled",
		Contacts:        []string{"kamesan@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{forceEnableAutoFraming: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})
}

// DebugParams defines some useful flags for debug CCA tests.
type DebugParams struct {
	SaveScreenshotWhenFail   bool
	SaveCameraFolderWhenFail bool
}

// TestWithAppParams defines parameters to control behaviors of running test
// with app.
type TestWithAppParams struct {
	StopAppOnlyIfExist bool
}

// ResetChromeFunc reset chrome used in this fixture.
type ResetChromeFunc func(context.Context) error

// StartAppFunc starts CCA.
type StartAppFunc func(context.Context) (*App, error)

// StopAppFunc stops CCA.
type StopAppFunc func(context.Context, bool) error

// ResetTestBridgeFunc resets the test bridge.
type ResetTestBridgeFunc func(context.Context) error

// TestWithAppFunc is the function to run with app.
type TestWithAppFunc func(context.Context, *App) error

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome      *chrome.Chrome
	BrowserType browser.Type
	ARC         *arc.ARC
	TestBridge  func() *testutil.TestBridge
	// App returns the CCA instance which lives through the test.
	App func() *App
	// ResetChrome resets chrome used by this fixture.
	ResetChrome ResetChromeFunc
	// StartApp starts CCA which can be used between subtests.
	StartApp StartAppFunc
	// StopApp stops CCA which can be used between subtests.
	StopApp StopAppFunc
	// ResetTestBridgeFunc resets the test bridge. Usually we don't need to call
	// it explicitly unless the sub test launch/tear-down the app itself.
	ResetTestBridge ResetTestBridgeFunc
	// SwitchScene switches the camera scene to the given scene. This only works
	// for fixtures using fake camera stream.
	SwitchScene func(string) error
	// RunTestWithApp runs the given function with the handling of the app
	// start/stop.
	RunTestWithApp func(context.Context, TestWithAppFunc, TestWithAppParams) error
	// PrepareChart prepares chart by loading the given scene. It only works for
	// CameraBox.
	PrepareChart func(ctx context.Context, addr, keyFile, contentPath string) error
	// SetDebugParams sets the debug parameters for current test.
	SetDebugParams func(params DebugParams)
}

type fixture struct {
	cr            *chrome.Chrome
	arc           *arc.ARC
	tb            *testutil.TestBridge
	app           *App
	outDir        string
	chart         *chart.Chart
	cameraScene   string
	brightnessVal string

	lacros                 bool
	scriptPaths            []string
	fakeCamera             bool
	fakeScene              bool
	arcBooted              bool
	launchCCA              bool
	bypassPermission       bool
	forceClamshell         bool
	guestMode              bool
	launchCCAInCameraBox   bool
	forceEnableAutoFraming bool
	debugParams            DebugParams
	features               []feature
}

func (f *fixture) cameraType() testutil.UseCameraType {
	if f.fakeCamera {
		return testutil.UseFakeCamera
	}
	return testutil.UseRealCamera
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	var chromeOpts []chrome.Option
	for _, f := range f.features {
		chromeOpts = append(chromeOpts, chrome.EnableFeatures(string(f)))
	}

	// Always enable doc scan DLC flag for testing.
	// TODO(b/226262670): Remove this line once the doc scan DLC is completely enabled.
	chromeOpts = append(chromeOpts, chrome.EnableFeatures("CameraAppDocScanDlc"))

	if f.fakeCamera {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs(
			// The default fps of fake device is 20, but CCA requires fps >= 24.
			// Set the fps to 30 to avoid OverconstrainedError.
			"--use-fake-device-for-media-stream=fps=30"))

		if f.fakeScene {
			dataDir := filepath.Dir(s.DataPath("cca_ui.js"))
			f.cameraScene = filepath.Join(dataDir, "camera_scene.mjpeg")
			chromeOpts = append(chromeOpts, chrome.ExtraArgs(
				// Set the default camera scene as the input of the fake stream.
				// The content of the scene can be dynamically changed during tests.
				"--use-file-for-fake-video-capture="+f.cameraScene))
		}
	}
	if f.guestMode {
		chromeOpts = append(chromeOpts, chrome.GuestLogin())
	}
	if f.arcBooted {
		chromeOpts = append(chromeOpts, chrome.ARCEnabled(), chrome.ExtraArgs("--disable-features=ArcResizeLock"))
	}
	if f.bypassPermission {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs("--use-fake-ui-for-media-stream"))
	}
	if f.forceClamshell {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}
	if f.forceEnableAutoFraming {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs("--auto-framing-override=force-enabled"))
	}

	// Enable assistant verbose logging for the CCAUIAssistant test. Since
	// assistant is disabled by default, this should not affect other tests.
	chromeOpts = append(chromeOpts, assistant.VerboseLogging())

	browserType := browser.TypeAsh
	if f.lacros {
		browserType = browser.TypeLacros
		var err error
		chromeOpts, err = lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chromeOpts...)).Opts()
		if err != nil {
			s.Fatal("Failed to compute Chrome options: ", err)
		}
	}

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	f.cr = cr
	defer func() {
		if !success {
			f.cr.Close(ctx)
			f.cr = nil
		}
	}()

	if f.arcBooted {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		f.arc = a
		defer func() {
			if !success {
				f.arc.Close(ctx)
				f.arc = nil
			}
		}()
	}
	if f.launchCCAInCameraBox {
		f.brightnessVal, err = dutcontrol.CCADimBacklight(ctx)
		if err != nil {
			s.Fatal("Failed to set brightness: ", err)
		}
		defer func() {
			if !success {
				dutcontrol.CCARestoreBacklight(ctx, f.brightnessVal)
			}
		}()
	}

	tb, err := testutil.NewTestBridge(ctx, cr, f.cameraType())
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	f.tb = tb
	f.scriptPaths = []string{s.DataPath("cca_ui.js")}

	success = true
	return FixtureData{
		Chrome:          f.cr,
		BrowserType:     browserType,
		ARC:             f.arc,
		TestBridge:      f.testBridge,
		App:             f.cca,
		ResetChrome:     f.resetChrome,
		StartApp:        f.startApp,
		StopApp:         f.stopApp,
		ResetTestBridge: f.resetTestBridge,
		SwitchScene:     f.switchScene,
		RunTestWithApp:  f.runTestWithApp,
		PrepareChart:    f.prepareChart,
		SetDebugParams:  f.setDebugParams,
	}
}

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.tb.TearDown(ctx); err != nil {
		s.Error("Failed to tear down test bridge: ", err)
	}
	f.tb = nil

	if f.arcBooted {
		if err := f.arc.Close(ctx); err != nil {
			s.Error("Failed to tear down ARC: ", err)
		}
		f.arc = nil
	}

	if err := f.cr.Close(ctx); err != nil {
		s.Error("Failed to tear down Chrome: ", err)
	}
	f.cr = nil
	if f.launchCCAInCameraBox {
		if err := dutcontrol.CCARestoreBacklight(ctx, f.brightnessVal); err != nil {
			s.Error("Restore Backlight failed: ", err)
		}
	}
	if f.cameraScene != "" {
		if err := os.RemoveAll(f.cameraScene); err != nil {
			s.Error("Failed to remove camera scene: ", err)
		}
		f.cameraScene = ""
	}
}

func (f *fixture) Reset(ctx context.Context) error {
	return f.resetTestBridge(ctx)
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	f.outDir = s.OutDir()
	if f.launchCCA {
		app, err := f.startApp(ctx)
		if err != nil {
			s.Fatal("Failed to start app: ", err)
		}
		f.app = app
	}
}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	defer func() {
		f.debugParams = DebugParams{}
	}()

	if f.chart != nil {
		if err := f.chart.Close(ctx, f.outDir); err != nil {
			s.Error("Failed to close chart: ", err)
		}
		f.chart = nil
	}

	if f.launchCCA {
		if err := f.stopApp(ctx, s.HasError()); err != nil {
			s.Fatal("Failed to stop app: ", err)
		}
	}
}

func (f *fixture) resetTestBridge(ctx context.Context) error {
	if err := f.tb.TearDown(ctx); err != nil {
		return errors.Wrap(err, "failed to tear down test bridge")
	}
	f.tb = nil

	cameraType := testutil.UseRealCamera
	if f.fakeCamera {
		cameraType = testutil.UseFakeCamera
	}
	tb, err := testutil.NewTestBridge(ctx, f.cr, cameraType)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	f.tb = tb
	return nil
}

func (f *fixture) resetChrome(ctx context.Context) error {
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome in fixture")
	}
	tb, err := testutil.NewTestBridge(ctx, f.cr, f.cameraType())
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge after reset chrome state")
	}
	f.tb = tb
	return nil
}

func (f *fixture) startApp(ctx context.Context) (*App, error) {
	if err := ClearSavedDir(ctx, f.cr); err != nil {
		return nil, errors.Wrap(err, "failed to clear camera folder")
	}

	app, err := New(ctx, f.cr, f.scriptPaths, f.outDir, f.tb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open CCA")
	}
	f.app = app
	return f.app, nil
}

func (f *fixture) stopApp(ctx context.Context, hasError bool) (retErr error) {
	if f.app == nil {
		return
	}

	defer func(ctx context.Context) {
		if err := f.app.CloseWithDebugParams(ctx, f.debugParams); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
		f.app = nil
	}(ctx)

	if hasError && f.debugParams.SaveScreenshotWhenFail {
		if err := f.app.SaveScreenshot(ctx); err != nil {
			return errors.Wrap(err, "failed to save a screenshot")
		}
	}
	return nil
}

func (f *fixture) stopAppIfExist(ctx context.Context, hasError bool) error {
	if appExist, err := InstanceExists(ctx, f.cr); err != nil {
		return errors.Wrap(err, "failed to check existence of CCA")
	} else if appExist {
		return f.stopApp(ctx, hasError)
	} else {
		// The app does not exist, might be proactively closed by the test flow.
		f.app = nil
	}
	return nil
}

// switchScene switches the camera scene of fake camera to the given |scene|.
func (f *fixture) switchScene(scene string) error {
	if f.cameraScene == "" {
		return errors.New("failed to switch scene for non-fake camera stream")
	}

	if err := fsutil.CopyFile(scene, f.cameraScene); err != nil {
		return errors.Wrapf(err, "failed to copy from the given scene: %v", scene)
	}
	return nil
}

func (f *fixture) runTestWithApp(ctx context.Context, testFunc TestWithAppFunc, params TestWithAppParams) (retErr error) {
	app, err := f.startApp(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start app")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	defer f.resetTestBridge(cleanupCtx)
	defer func(cleanupCtx context.Context) {
		hasError := retErr != nil
		stopFunc := f.stopApp
		if params.StopAppOnlyIfExist {
			stopFunc = f.stopAppIfExist
		}
		if err := stopFunc(cleanupCtx, hasError); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(cleanupCtx)

	return testFunc(ctx, app)
}

func (f *fixture) prepareChart(ctx context.Context, addr, keyFile, contentPath string) (retErr error) {
	var sopt ssh.Options
	ssh.ParseTarget(addr, &sopt)
	sopt.KeyFile = keyFile
	sopt.ConnectTimeout = 10 * time.Second
	conn, err := ssh.New(ctx, &sopt)
	if err != nil {
		return errors.Wrap(err, "failed to connect to chart tablet")
	}
	// No need to close ssh connection since chart will handle it when cleaning
	// up.

	c, namePaths, err := chart.SetUp(ctx, conn, f.outDir, []string{contentPath})
	if err != nil {
		return errors.Wrap(err, "failed to prepare chart tablet")
	}

	if err := c.Display(ctx, namePaths[0]); err != nil {
		return errors.Wrap(err, "failed to display chart on chart tablet")
	}

	f.chart = c
	return nil
}

func (f *fixture) testBridge() *testutil.TestBridge {
	return f.tb
}

func (f *fixture) cca() *App {
	return f.app
}

func (f *fixture) setDebugParams(params DebugParams) {
	f.debugParams = params
}
