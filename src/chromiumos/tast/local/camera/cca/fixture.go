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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	ccaSetUpTimeout    = 15 * time.Second
	ccaTearDownTimeout = 5 * time.Second
	setUpTimeout       = chrome.LoginTimeout + ccaSetUpTimeout
	tearDownTimeout    = chrome.ResetTimeout
)

const (
	qrcodeScene     string = "qrcode_1280x960.y4m"
	qrcodeTextScene        = "qrcode_text_1280x960.y4m"
	documentScene          = "document_1280x960.y4m"
	genPTZScene            = "GENERATED_PTZ_SCENE"
)

const (
	y4mWidth      = 1280
	y4mHeight     = 720
	patternWidth  = 101
	patternHeight = 101
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunched",
		Desc:            "Launched CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedGuest",
		Desc:            "Launched CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{guestMode: true, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithFakeCamera",
		Desc:            "Launched CCA with fake camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithQRCodeUrlScene",
		Desc:            "Launched CCA with QR code URL as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", qrcodeScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: qrcodeScene, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithQRCodeTextScene",
		Desc:            "Launched CCA with QR code text as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", qrcodeTextScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: qrcodeTextScene, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaLaunchedWithPTZScene",
		Desc:            "Launched CCA with generated scene for PTZ tests as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: genPTZScene, launchCCA: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
		PreTestTimeout:  ccaSetUpTimeout,
		PostTestTimeout: ccaTearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReady",
		Desc:            "Set up test bridge for CCA",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyBypassPermissionClamshell",
		Desc:            "Set up test bridge for CCA with bypassPermission on clamshell mode on",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{bypassPermission: true, forceClamshell: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyWithArc",
		Desc:            "Set up test bridge for CCA with ARC enabled",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js"},
		Impl:            &fixture{arcBooted: true},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "ccaTestBridgeReadyWithDocumentScene",
		Desc:            "Set up test bridge for CCA with document scene as camera input",
		Contacts:        []string{"wtlee@chromium.org"},
		Data:            []string{"cca_ui.js", documentScene},
		Impl:            &fixture{fakeCamera: true, fakeCameraFile: documentScene},
		SetUpTimeout:    setUpTimeout,
		TearDownTimeout: tearDownTimeout,
	})
}

// DebugParams defines some useful flags for debug CCA tests.
type DebugParams struct {
	SaveScreenshotWhenFail   bool
	SaveCameraFolderWhenFail bool
}

// FixtureData is the struct exposed to tests.
type FixtureData struct {
	Chrome     *chrome.Chrome
	ARC        *arc.ARC
	TestBridge *testutil.TestBridge
	// App returns the CCA instance which lives through the test.
	App func() *App
	// StartApp starts CCA which can be used between subtests.
	StartApp func(ctx context.Context) (*App, error)
	// StopApp stops CCA which can be used between subtests.
	StopApp func(ctx context.Context, hasError bool) error
	// StopAppIfExist works similar to StopApp, but only closes the app if the
	// app exists. It is useful for tests which will proactively close the app
	// in the test flow.
	StopAppIfExist func(ctx context.Context, hasError bool) error
	// SetDebugParams sets the debug parameters for current test.
	SetDebugParams func(params DebugParams)
}

type fixture struct {
	cr     *chrome.Chrome
	arc    *arc.ARC
	tb     *testutil.TestBridge
	app    *App
	outDir string

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
}

// StartAppFunc starts CCA.
type StartAppFunc func(context.Context) (*App, error)

// StopAppFunc stops CCA.
type StopAppFunc func(context.Context, bool) error

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	var chromeOpts []chrome.Option
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
				a.Close(ctx)
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
	return FixtureData{Chrome: f.cr, ARC: f.arc, TestBridge: f.tb,
		App:            f.cca,
		StartApp:       f.startApp,
		StopApp:        f.stopApp,
		StopAppIfExist: f.stopAppIfExist,
		SetDebugParams: f.setDebugParams}
}

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.genScene != "" {
		if err := os.Remove(f.genScene); err != nil {
			s.Error("Failed to remove generated scene: ", err)
		}
		f.genScene = ""
	}

	if err := f.tb.TearDown(ctx); err != nil {
		s.Fatal("Failed to tear down test bridge: ", err)
	}
	f.tb = nil
}

func (f *fixture) Reset(ctx context.Context) error {
	return nil
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

	if f.launchCCA {
		if err := f.stopApp(ctx, s.HasError()); err != nil {
			s.Fatal("Failed to stop app: ", err)
		}
	}
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

func (f *fixture) cca() *App {
	return f.app
}

func (f *fixture) setDebugParams(params DebugParams) {
	f.debugParams = params
}

// genFileForPTZ generates the fake preview y4m file and returns its name.
func genFileForPTZ() (_ string, retErr error) {
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
