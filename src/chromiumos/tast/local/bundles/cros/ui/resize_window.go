// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"math"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type appSource int

const (
	appBuiltin appSource = 1 << iota
	appCws
	appArc
)

type resizeDragSourcePosition int

const (
	topLeft resizeDragSourcePosition = iota
	topRight
	bottomLeft
	bottomRight
	left
	right
	top
	bottom
	resizeDragSourcePositionEnd
)

const (
	arcAppInstallationTimeout = 5 * time.Minute

	// defaultMargin indicates the distance in pixels outside the border of the "normal" window from which it should be grabbed.
	defaultMargin = 5
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ResizeWindow,
		Desc: "Resize different windows by dragging 4 corners and 4 sides",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{
			{
				Val:     appBuiltin | appCws,
				Timeout: 5*time.Minute + cws.InstallationTimeout,
			}, {
				Name:              "arc",
				Val:               appArc,
				ExtraSoftwareDeps: []string{"arc"},
				Timeout:           5*time.Minute + arcAppInstallationTimeout,
			}},
	})
}

// ResizeWindow tests that resizing windows by dragging 4 corners and 4 sides.
func ResizeWindow(ctx context.Context, s *testing.State) {
	var (
		source   = s.Param().(appSource)
		isTestOn = func(as appSource) bool { return source&as != 0 }
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var opts = []chrome.Option{}
	if isTestOn(appArc) {
		opts = append(opts, chrome.ARCSupported())
		opts = append(opts, chrome.ExtraArgs(arc.DisableSyncFlags()...))
		opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	}

	// Avoid share session with other tests to ensure the window size is in initial state.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to Chrome login: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	testUtil, err := newUtil(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to prepare for resize window: ", err)
	}

	var appList []app
	if isTestOn(appArc) {
		if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to opt into Play Store: ", err)
		}

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(cleanupCtx)

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed initializing UI Automator: ", err)
		}
		defer d.Close(cleanupCtx)

		appList = append(appList, newArcAppYtMusic(cr, a, d, tconn, testUtil.ui))
	}
	if isTestOn(appCws) {
		appList = append(appList, newCwsAppText(cr, tconn, kb))
	}
	if isTestOn(appBuiltin) {
		chromeApp, err := newBuiltinAppChrome(ctx, tconn, kb)
		if err != nil {
			s.Fatal("Failed to prepare built-in app Chrome: ", err)
		}
		appList = append(appList, newBuiltinAppFiles(tconn, kb))
		appList = append(appList, chromeApp)
	}

	for _, app := range appList {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubTestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			if err := app.install(ctx); err != nil {
				s.Fatalf("Failed to install app %q: %v", app.getAppName(), err)
			}
			defer app.unInstall(cleanupSubTestCtx)

			if err := app.launch(ctx); err != nil {
				s.Fatalf("Failed to launch app %q: %v", app.getAppName(), err)
			}
			defer app.close(cleanupSubTestCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, s.OutDir(), s.HasError, cr, app.getAppName())

			// Minimize the window in advance to avoid its border being off-screen after the window has been dragged to the center of screen.
			if err := testUtil.resizeWindowToMin(app.getWindowFinder())(ctx); err != nil {
				s.Fatal("Failed to minimize window: ", err)
			}
			s.Log("Window has been resized to minimize")

			// Move the window to the center of the screen to observe the test.
			if err := testUtil.moveWindowToCenter(app)(ctx); err != nil {
				s.Fatal("Failed to center window: ", err)
			}
			s.Log("Window has been moved to the center of screen")

			for i := 0; i < int(resizeDragSourcePositionEnd); i++ {
				if err := testUtil.resizeWindow(app.getWindowFinder(), resizeDragSourcePosition(i))(ctx); err != nil {
					s.Fatal("Failed to resize window: ", err)
				}
			}
		}

		if !s.Run(ctx, fmt.Sprintf("resize on app %s", app.getAppName()), f) {
			s.Error("Failed to test resize functionality on app ", app.getAppName())
		}
	}
}

type resizeWindowUtil struct {
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	resizeArea coords.Rect
}

func newUtil(ctx context.Context, tconn *chrome.TestConn) (*resizeWindowUtil, error) {
	ui := uiauto.New(tconn)

	rootWindowFinder := nodewith.HasClass("RootWindow-0").Role(role.Window)
	resizeArea, err := ui.Info(ctx, rootWindowFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root window info")
	}

	shelfInfo, err := ui.Info(ctx, nodewith.Role(role.Toolbar).ClassName("ShelfView"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get shelf info")
	}
	resizeArea.Location.Height -= shelfInfo.Location.Height

	return &resizeWindowUtil{
		tconn:      tconn,
		ui:         ui,
		resizeArea: resizeArea.Location,
	}, nil
}

// stableDrag drags the window and waits for location be stabled.
func (t *resizeWindowUtil) stableDrag(node *nodewith.Finder, srcPt, endPt coords.Point) uiauto.Action {
	return uiauto.Combine("mouse drag and wait for location be stabled",
		t.ui.WaitForLocation(node),
		mouse.Drag(t.tconn, srcPt, endPt, time.Second),
		t.ui.WaitForLocation(node),
	)
}

// resizeWindowToMin minimizes window by dragging form top-left to bottom-right of the window.
func (t *resizeWindowUtil) resizeWindowToMin(f *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		rect, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		return t.stableDrag(f, rect.Location.TopLeft(), rect.Location.BottomRight())(ctx)
	}
}

// moveWindowToCenter places the window on the center of screen.
func (t *resizeWindowUtil) moveWindowToCenter(app app) uiauto.Action {
	return func(ctx context.Context) error {
		f := app.getWindowFinder()

		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfo, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info")
		}

		src := coords.NewPoint(windowInfo.Location.CenterX(), windowInfo.Location.Top+defaultMargin)
		if _, ok := app.(*arcAppYtMusic); ok {
			centerBtnInfo, err := t.ui.Info(ctx, nodewith.Name("Resizable").HasClass("FrameCenterButton"))
			if err != nil {
				testing.ContextLog(ctx, "Failed to get center button of title bar info")
			} else {
				// Drag the left side of center button of the title bar to avoid moving failure.
				src = coords.Point{X: centerBtnInfo.Location.Left - defaultMargin, Y: windowInfo.Location.Top + defaultMargin}
			}
		}
		dest := coords.NewPoint(t.resizeArea.CenterX(), t.resizeArea.CenterY()-windowInfo.Location.Height/2)

		return t.stableDrag(f, src, dest)(ctx)
	}
}

// resizeWindow resizes window by dragging corners/sides.
func (t *resizeWindowUtil) resizeWindow(f *nodewith.Finder, srcPos resizeDragSourcePosition) uiauto.Action {
	return func(ctx context.Context) error {
		if err := t.ui.WaitForLocation(f)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for window to be stable")
		}

		windowInfoBefore, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info before resize")
		}

		start, end := t.dragDetail(srcPos, windowInfoBefore)
		if err := t.stableDrag(f, start, end)(ctx); err != nil {
			return errors.Wrap(err, "failed to resize")
		}

		windowInfoAfter, err := t.ui.Info(ctx, f)
		if err != nil {
			return errors.Wrap(err, "failed to get window info after resize")
		}

		if err := t.verifyLocation(windowInfoAfter.Location, end); err != nil {
			return errors.New("failed to verify that the window has been resized")
		}
		testing.ContextLog(ctx, "Window has resized as expected")

		if err := t.stableDrag(f, end, start)(ctx); err != nil {
			return errors.Wrap(err, "failed to restore resize")
		}

		return nil
	}
}

// verifyLocation verifies that the new position of window is correct.
func (t *resizeWindowUtil) verifyLocation(windowLoc coords.Rect, expectedPos coords.Point) error {
	// The difference between each bound of target window's location and expected position must be less than default margin.
	if math.Abs(float64(windowLoc.Left)-float64(expectedPos.X)) > defaultMargin &&
		math.Abs(float64(windowLoc.Right())-float64(expectedPos.X)) > defaultMargin &&
		math.Abs(float64(windowLoc.Top)-float64(expectedPos.Y)) > defaultMargin &&
		math.Abs(float64(windowLoc.Bottom())-float64(expectedPos.Y)) > defaultMargin {
		return errors.New("window location is not as expected")
	}
	return nil
}

// dragDetail adjusts the position that should be dragged according to defaultMargin.
func (t *resizeWindowUtil) dragDetail(srcPos resizeDragSourcePosition, nodeToResize *uiauto.NodeInfo) (sourcePt, endPt coords.Point) {
	var shift coords.Point
	switch srcPos {
	case topLeft:
		shift = coords.NewPoint(-defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopLeft()
		endPt = t.resizeArea.TopLeft()
	case topRight:
		shift = coords.NewPoint(defaultMargin, -defaultMargin)
		sourcePt = nodeToResize.Location.TopRight()
		endPt = t.resizeArea.TopRight()
	case bottomLeft:
		shift = coords.NewPoint(-defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomLeft()
		endPt = t.resizeArea.BottomLeft()
	case bottomRight:
		shift = coords.NewPoint(defaultMargin, defaultMargin)
		sourcePt = nodeToResize.Location.BottomRight()
		endPt = t.resizeArea.BottomRight()
	case left:
		shift = coords.NewPoint(-defaultMargin, 0)
		sourcePt = nodeToResize.Location.LeftCenter()
		endPt = t.resizeArea.LeftCenter()
		endPt.Y = sourcePt.Y
	case right:
		shift = coords.NewPoint(defaultMargin, 0)
		sourcePt = nodeToResize.Location.RightCenter()
		endPt = t.resizeArea.RightCenter()
		endPt.Y = sourcePt.Y
	case top:
		shift = coords.NewPoint(0, -defaultMargin)
		sourcePt = coords.NewPoint(nodeToResize.Location.CenterX(), nodeToResize.Location.Top)
		endPt = coords.NewPoint(t.resizeArea.CenterX(), t.resizeArea.Top)
		endPt.X = sourcePt.X
	case bottom:
		shift = coords.NewPoint(0, defaultMargin)
		sourcePt = nodeToResize.Location.BottomCenter()
		endPt = t.resizeArea.BottomCenter()
		endPt.X = sourcePt.X
	}
	return sourcePt.Add(shift), endPt.Sub(shift)
}

type app interface {
	install(ctx context.Context) error
	unInstall(ctx context.Context) error
	launch(ctx context.Context) error
	close(ctx context.Context) error

	getAppName() string
	getWindowFinder() *nodewith.Finder
}

type cwsAppText struct {
	cws.App
	id           string
	windowFinder *nodewith.Finder

	cr    *chrome.Chrome
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
}

func newCwsAppText(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *cwsAppText {
	return &cwsAppText{
		App: cws.App{
			Name: "Text",
			URL:  "https://chrome.google.com/webstore/detail/text/mmfbcljfglbokpmkimbfghdkjmjhdgbg",
		},
		id:           "mmfbcljfglbokpmkimbfghdkjmjhdgbg",
		windowFinder: nodewith.HasClass("RootView").Name("Text"),
		cr:           cr,
		tconn:        tconn,
		kb:           kb,
	}
}

func (app *cwsAppText) install(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, app.tconn, app.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existence")
	}

	if isInstalled {
		return nil
	}
	return cws.InstallApp(ctx, app.cr, app.tconn, app.App)
}

func (app *cwsAppText) launch(ctx context.Context) error {
	return launcher.LaunchApp(app.tconn, app.Name)(ctx)
}

func (app *cwsAppText) close(ctx context.Context) error {
	return apps.Close(ctx, app.tconn, app.id)
}

func (app *cwsAppText) unInstall(ctx context.Context) error {
	defer func() {
		settings := ossettings.New(app.tconn)
		settings.Close(ctx)
	}()
	return ossettings.UninstallApp(ctx, app.tconn, app.cr, app.Name, app.id)
}

func (app *cwsAppText) getAppName() string                { return app.Name }
func (app *cwsAppText) getWindowFinder() *nodewith.Finder { return app.windowFinder }

type arcAppYtMusic struct {
	name         string
	id           string
	pkgName      string
	windowFinder *nodewith.Finder

	cr    *chrome.Chrome
	a     *arc.ARC
	d     *androidui.Device
	tconn *chrome.TestConn
	ui    *uiauto.Context
}

func newArcAppYtMusic(cr *chrome.Chrome, a *arc.ARC, d *androidui.Device, tconn *chrome.TestConn, ui *uiauto.Context) *arcAppYtMusic {
	return &arcAppYtMusic{
		name:         "YT Music",
		id:           "hpdkdmlckojaocbedhffglopeafcgggc",
		pkgName:      "com.google.android.apps.youtube.music",
		windowFinder: nodewith.HasClass("RootView").Name("YT Music"),

		a:     a,
		d:     d,
		cr:    cr,
		tconn: tconn,
		ui:    ui,
	}
}

func (app *arcAppYtMusic) install(ctx context.Context) error {
	installCtx, cancelInstall := context.WithTimeout(ctx, 3*time.Minute)
	defer cancelInstall()

	// playstore.InstallApp installs the app if the app is not installed.
	if err := playstore.InstallApp(installCtx, app.a, app.d, app.pkgName, -1); err != nil {
		return errors.Wrapf(err, "failed to install arc app %s", app.name)
	}

	if err := optin.ClosePlayStore(ctx, app.tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}

	return nil
}

func (app *arcAppYtMusic) launch(ctx context.Context) error {
	if err := launcher.LaunchAndWaitForAppOpen(app.tconn, apps.App{ID: app.id, Name: app.name})(ctx); err != nil {
		return err
	}
	return app.setWindowMode(ctx)
}

func (app *arcAppYtMusic) close(ctx context.Context) error {
	return apps.Close(ctx, app.tconn, app.id)
}

// setWindowMode sets the mode of YT music from `Tablet` to `Resizable`.
func (app *arcAppYtMusic) setWindowMode(ctx context.Context) error {
	conn, err := apps.LaunchOSSettings(ctx, app.cr, "chrome://os-settings/app-management/detail?id="+app.id)
	if err != nil {
		return errors.Wrap(err, "failed to open settings of yt music")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	btnResizeLock := nodewith.Name("Preset window sizes")
	if err := app.ui.WaitUntilExists(btnResizeLock)(ctx); err != nil {
		return nil
	}
	return app.ui.LeftClick(btnResizeLock)(ctx)
}

func (app *arcAppYtMusic) unInstall(ctx context.Context) error {
	return app.a.Uninstall(ctx, app.pkgName)
}

func (app *arcAppYtMusic) getAppName() string                { return app.name }
func (app *arcAppYtMusic) getWindowFinder() *nodewith.Finder { return app.windowFinder }

type builtinApp struct {
	name         string
	id           string
	windowFinder *nodewith.Finder

	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
}

func newBuiltinApp(app apps.App, windowFinder *nodewith.Finder, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *builtinApp {
	return &builtinApp{
		name:         app.Name,
		id:           app.ID,
		windowFinder: windowFinder,
		tconn:        tconn,
		kb:           kb,
	}
}

func (app *builtinApp) launch(ctx context.Context) error {
	return launcher.LaunchApp(app.tconn, app.name)(ctx)
}

func (app *builtinApp) close(ctx context.Context) error {
	return apps.Close(ctx, app.tconn, app.id)
}

func (app *builtinApp) install(ctx context.Context) error   { return nil }
func (app *builtinApp) unInstall(ctx context.Context) error { return nil }
func (app *builtinApp) getAppName() string                  { return app.name }
func (app *builtinApp) getWindowFinder() *nodewith.Finder   { return app.windowFinder }

func newBuiltinAppChrome(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) (*builtinApp, error) {
	c, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check installed chrome browser")
	}
	return newBuiltinApp(c, nodewith.HasClass("NonClientView").NameContaining(c.Name), tconn, kb), nil
}

func newBuiltinAppFiles(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *builtinApp {
	return newBuiltinApp(apps.Files, filesapp.WindowFinder(apps.Files.ID), tconn, kb)
}
