// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	ccaSetUpTimeout        = 25 * time.Second
	ccaTearDownTimeout     = 5 * time.Second
	testBridgeSetUpTimeout = 5 * time.Second
	setUpTimeout           = chrome.LoginTimeout + testBridgeSetUpTimeout
	tearDownTimeout        = chrome.ResetTimeout
)

const (
	qrcodeScene     string = "qrcode_1280x960.y4m"
	qrcodeTextScene        = "qrcode_text_1280x960.y4m"
	documentScene          = "two_document_3264x2448_20211020.mjpeg"
	genPTZScene            = "GENERATED_PTZ_SCENE"
)

type feature string

const (
	manualCrop feature = "CameraAppDocumentManualCrop"
)

func init() {
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
		Name:            "ccaLaunchedWithQRCodeUrlScene",
		Desc:            "Launched CCA with QR code URL as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", qrcodeScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: qrcodeScene, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithQRCodeTextScene",
		Desc:            "Launched CCA with QR code text as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", qrcodeTextScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: qrcodeTextScene, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithPTZScene",
		Desc:            "Launched CCA with generated scene for PTZ tests as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: genPTZScene, launchCCA: true},
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
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyWithDocumentScene",
		Desc:            "Set up test bridge for CCA with document scene as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", documentScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: documentScene},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    testBridgeSetUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "ccaTestBridgeReadyForDocumentManualCrop",
		Desc:     "Set up test bridge for CCA and chrome for testing document manual crop",
		Contacts: []string{"inker@chromium.org"},
		Data:     []string{"cca_ui.js", documentScene},
		Impl: &fixture{
			fakeCamera:     true,
			fakeCameraFile: documentScene,
			features:       []feature{manualCrop},
		},
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

// SubTestParams defines parameters to control behaviors of running subtest.
type SubTestParams struct {
	StopAppOnlyIfExist bool
}

// StartAppFunc starts CCA.
type StartAppFunc func(context.Context) (*App, error)

// StopAppFunc stops CCA.
type StopAppFunc func(context.Context, bool) error

// SubTestFunc is the function to run in a sub test.
type SubTestFunc func(context.Context, *App) error

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome     *chrome.Chrome
	ARC        *arc.ARC
	TestBridge func() *testutil.TestBridge
	// App returns the CCA instance which lives through the test.
	App func() *App
	// StartApp starts CCA which can be used between subtests.
	StartApp StartAppFunc
	// StopApp stops CCA which can be used between subtests.
	StopApp StopAppFunc
	// RubSubTest runs the given function as a sub test, handling the app
	// start/stop it.
	RunSubTest func(context.Context, SubTestFunc, SubTestParams) error
	// PrepareChart prepares chart by loading the given scene. It only works for
	// CameraBox.
	PrepareChart func(ctx context.Context, addr, keyFile, contentPath string) error
	// SetDebugParams sets the debug parameters for current test.
	SetDebugParams func(params DebugParams)
}

type fixture struct {
	cr     *chrome.Chrome
	arc    *arc.ARC
	tb     *testutil.TestBridge
	app    *App
	outDir string
	chart  *chart.Chart

	scriptPaths      []string
	fakeCamera       bool
	fakeCameraFile   string
	arcBooted        bool
	launchCCA        bool
	bypassPermission bool
	forceClamshell   bool
	guestMode        bool
	genScene         string
	debugParams      DebugParams
	features         []feature
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	var chromeOpts []chrome.Option
	for _, f := range f.features {
		chromeOpts = append(chromeOpts, chrome.EnableFeatures(string(f)))
	}
	if f.fakeCamera {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs(
			// The default fps of fake device is 20, but CCA requires fps >= 24.
			// Set the fps to 30 to avoid OverconstrainedError.
			"--use-fake-device-for-media-stream=fps=30"))
		if f.fakeCameraFile == genPTZScene {
			genFile, err := genFileForPTZ()
			if err != nil {
				s.Fatal("Failed to get generated file for PTZ tests: ", err)
			}
			f.genScene = genFile
			defer func() {
				if !success {
					os.Remove(f.genScene)
					f.genScene = ""
				}
			}()
			chromeOpts = append(chromeOpts,
				chrome.ExtraArgs("--use-file-for-fake-video-capture="+f.genScene))
		} else if f.fakeCameraFile != "" {
			chromeOpts = append(chromeOpts,
				chrome.ExtraArgs("--use-file-for-fake-video-capture="+s.DataPath(f.fakeCameraFile)))
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

	cameraType := testutil.UseRealCamera
	if f.fakeCamera {
		cameraType = testutil.UseFakeCamera
	}
	tb, err := testutil.NewTestBridge(ctx, cr, cameraType)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	f.tb = tb
	f.scriptPaths = []string{s.DataPath("cca_ui.js")}

	success = true
	return FixtureData{Chrome: f.cr, ARC: f.arc,
		TestBridge:     f.testBridge,
		App:            f.cca,
		StartApp:       f.startApp,
		StopApp:        f.stopApp,
		RunSubTest:     f.runSubTest,
		PrepareChart:   f.prepareChart,
		SetDebugParams: f.setDebugParams}
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

	if f.genScene != "" {
		if err := os.Remove(f.genScene); err != nil {
			s.Error("Failed to remove generated scene: ", err)
		}
		f.genScene = ""
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

func (f *fixture) runSubTest(ctx context.Context, subTestFunc SubTestFunc, params SubTestParams) (retErr error) {
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

	return subTestFunc(ctx, app)
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

	c, err := chart.SetUp(ctx, conn, contentPath, f.outDir)
	if err != nil {
		return errors.Wrap(err, "failed to prepare chart tablet")
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

// genFileForPTZ generates the fake preview y4m file and returns its name.
func genFileForPTZ() (_ string, retErr error) {
	y4mWidth := 1280
	y4mHeight := 720
	patternWidth := 101
	patternHeight := 101

	file, err := ioutil.TempFile(os.TempDir(), "*.y4m")
	if err != nil {
		return "", err
	}
	defer func() {
		if retErr != nil {
			os.Remove(file.Name())
		}
	}()

	header := fmt.Sprintf("YUVMPEG2 W%d H%d F30:1 Ip A0:0 C420jpeg\nFRAME\n", y4mWidth, y4mHeight)
	if _, err := file.WriteString(header); err != nil {
		return "", errors.Wrap(err, "failed to write header of temp y4m")
	}

	// White background.
	const (
		bgY = 255
		bgU = 128
		bgV = 128
	)

	// Y plane.
	yp := make([][]byte, y4mHeight)
	for y := range yp {
		yp[y] = make([]byte, y4mWidth)
		for x := range yp[y] {
			yp[y][x] = bgY
		}
	}

	// Draws black square pattern at the center.
	cy := y4mHeight / 2
	cx := y4mWidth / 2
	for dy := -patternHeight / 2; dy <= patternHeight/2; dy++ {
		for dx := -patternWidth / 2; dx <= patternWidth/2; dx++ {
			yp[cy+dy][cx+dx] = 0
		}
	}

	for _, bs := range yp {
		if _, err := file.Write(bs); err != nil {
			return "", errors.Wrap(err, "failed to write Y plane of temp y4m")
		}
	}

	// U plane.
	up := make([]byte, y4mWidth*y4mHeight/4)
	for x := 0; x < len(up); x++ {
		up[x] = bgU
	}
	if _, err := file.Write(up); err != nil {
		return "", errors.Wrap(err, "failed to write U plane of temp y4m")
	}

	// V plane.
	vp := make([]byte, y4mWidth*y4mHeight/4)
	for x := 0; x < len(vp); x++ {
		vp[x] = bgV
	}
	if _, err := file.Write(vp); err != nil {
		return "", errors.Wrap(err, "failed to write V plane of temp y4m")
	}

	if err := os.Chmod(file.Name(), 0644); err != nil {
		return "", err
	}

	return file.Name(), nil
}
