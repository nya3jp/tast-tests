// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tinkercad implements TinkerCAD web operations.
package tinkercad

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// TinkerCad defines the struct related to TinkerCAD website.
type TinkerCad struct {
	ui             *uiauto.Context
	ud             *uidetection.Context
	conn           *chrome.Conn
	tconn          *chrome.TestConn
	kb             *input.KeyboardEventWriter
	pc             *pointer.MouseContext
	EditorWinRect  coords.Rect
	exportFilePath string
	rotateIconPath string
	addShapeCount  int
}

// ViewMode defines several view mode's node finder.
type ViewMode *nodewith.Finder

var (
	// loginAreaFinder defines login window's node as finder.
	loginAreaFinder = nodewith.Ancestor(nodewith.Name("Login | Tinkercad").Role(role.RootWebArea))
	// dashboardAreaFinder defines dashboard window's node as finder.
	dashboardAreaFinder = nodewith.Ancestor(nodewith.Name("Dashboard | Tinkercad").Role(role.RootWebArea))
	// editorArea defines editor window's node as finder.
	// The editor window is the design editing area of TinkerCAD.
	editorArea = nodewith.HasClass("editorContainer").Role(role.GenericContainer)
	// editorAreaFinder defines editor window's node as finder.
	editorAreaFinder = nodewith.Ancestor(editorArea)
	// ViewHome defines home view's node finder as ViewMode.
	ViewHome ViewMode = editorAreaFinder.HasClass("home-view-container").Role(role.GenericContainer)
	// ViewFit defines fit view's node finder as ViewMode.
	ViewFit ViewMode = editorAreaFinder.HasClass("fit-view-container").Role(role.GenericContainer)
	// ViewZoomIn defines zoom in view's node finder as ViewMode.
	ViewZoomIn ViewMode = editorAreaFinder.HasClass("zoomin-view-container").Role(role.GenericContainer)
	// ViewZoomOut defines zoom out view's node finder as ViewMode.
	ViewZoomOut ViewMode = editorAreaFinder.HasClass("zoomout-view-container").Role(role.GenericContainer)
	// ViewPerspective defines perspective view's node finder as ViewMode.
	ViewPerspective ViewMode = editorAreaFinder.HasClass("persp-view-container").Role(role.GenericContainer)
)

const (
	// Used for draging the design.
	dragDuration = 3 * time.Second
	// Used for waiting the design being stable enough to do the next step.
	waitUIDuration = 3 * time.Second
	// Used for situations where UI response might be faster.
	shortUITimeout = 10 * time.Second
)

// NewTinkerCad creates an instance of TinkerCAD.
func NewTinkerCad(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, rotateIconPath string) *TinkerCad {
	return &TinkerCad{
		ui:             uiauto.New(tconn),
		ud:             uidetection.NewDefault(tconn),
		tconn:          tconn,
		kb:             kb,
		pc:             pointer.NewMouse(tconn),
		EditorWinRect:  coords.Rect{},
		rotateIconPath: rotateIconPath,
	}
}

// Open opens a TinkerCAD website on chrome browser.
func (tc *TinkerCad) Open(ctx context.Context, cr *chrome.Chrome) (err error) {
	tc.conn, err = cr.NewConn(ctx, cuj.TinkerCadDashboardURL)
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	return nil
}

// Login logs in to TinkerCAD with google oauth.
// Will do sign up process if don't have a TinkerCAD account.
func (tc *TinkerCad) Login(ctx context.Context, account string) error {
	if err := tc.conn.Navigate(ctx, cuj.TinkerCadSignInURL); err != nil {
		return errors.Wrapf(err, "failed to navigate to %s", cuj.TinkerCadSignInURL)
	}
	signInWithGoogleOauth := func(ctx context.Context) error {
		personalAccounts := loginAreaFinder.Name("Personal accounts").Role(role.StaticText)
		signIn := loginAreaFinder.Name("Sign in with Google").Role(role.StaticText)
		account := nodewith.Name(account).Role(role.StaticText)
		btnContinue := nodewith.Name("Continue").HasClass("adsk-btn").Role(role.Button)
		// If the DUT has only one account, it would login to TinkerCAD directly.
		// Otherwise, it would show list of accounts.
		// Also it will do sign up process if haven't done it before.
		return uiauto.NamedCombine("login to TinkerCAD",
			tc.ui.DoDefaultUntil(personalAccounts, tc.ui.WithTimeout(shortUITimeout).WaitUntilExists(signIn)),
			tc.ui.DoDefault(signIn),
			// Click account button while there's more than one google account.
			uiauto.IfSuccessThen(tc.ui.WaitUntilExists(account),
				tc.ui.DoDefaultUntil(account, tc.ui.Gone(account))),
			// Click continue button while the account hasn't signed up.
			uiauto.IfSuccessThen(tc.ui.WaitUntilExists(btnContinue),
				tc.ui.DoDefault(btnContinue)),
		)(ctx)
	}
	testing.ContextLogf(ctx, "Login to TinkerCAD though google oauth with %s", account)
	createNewDesign := dashboardAreaFinder.Name("Create new design").Role(role.StaticText)
	if err := tc.ui.WithTimeout(shortUITimeout).WaitUntilExists(createNewDesign)(ctx); err != nil {
		if err := signInWithGoogleOauth(ctx); err != nil {
			return errors.Wrap(err, "failed to sign in TinkerCAD with google oauth")
		}
	}
	testing.ContextLog(ctx, "Already login to TinkerCAD, going to reuse it")
	return nil
}

// Copy copies an initial design from sample design URL and returns the name of the design.
func (tc *TinkerCad) Copy(ctx context.Context, sampleDesignURL string) (string, error) {
	if err := tc.conn.Navigate(ctx, sampleDesignURL); err != nil {
		return "", errors.Wrapf(err, "failed to navigate to %q", sampleDesignURL)
	}
	btnCopy := nodewith.Name("Copy and Tinker").Role(role.Button)
	sampleDesign := nodewith.Role(role.Heading).Ancestor(nodewith.HasClass("title-container"))
	sampleDesignInfo, err := tc.ui.Info(ctx, sampleDesign)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get smaple design name from: %s", sampleDesignURL)
	}
	testing.ContextLogf(ctx, "Copy sample design %s from: %s", sampleDesignInfo.Name, sampleDesignURL)
	return sampleDesignInfo.Name, tc.ui.DoDefault(btnCopy)(ctx)
}

// Delete deletes the specific design by it's name.
func (tc *TinkerCad) Delete(ctx context.Context, sampleDesignName string) error {
	// Delete export file if design is exported.
	if tc.exportFilePath != "" {
		testing.ContextLogf(ctx, "Delete export design from path: %s", tc.exportFilePath)
		if err := os.Remove(tc.exportFilePath); err != nil {
			return errors.Wrap(err, "failed to remove a file")
		}
	}
	if err := tc.conn.Navigate(ctx, cuj.TinkerCadDashboardURL); err != nil {
		return errors.Wrapf(err, "failed to navigate to %q", cuj.TinkerCadDashboardURL)
	}
	checkBoxSelect := dashboardAreaFinder.Name("Select").Role(role.CheckBox)
	sampleDesign := dashboardAreaFinder.NameContaining(sampleDesignName).Role(role.StaticText).First()
	btnDelete := dashboardAreaFinder.NameContaining("Delete").Role(role.StaticText)
	popUpFinder := nodewith.Ancestor(nodewith.HasClass("modal-content").Role(role.GenericContainer))
	deletionCheckText := popUpFinder.NameContaining("Deleting an item permanently").Role(role.StaticText)
	btnFinalDelete := popUpFinder.Name("Delete").Role(role.Button).Focusable()
	return uiauto.NamedCombine("delete the design",
		tc.ui.DoDefaultUntil(checkBoxSelect, tc.ui.Exists(btnDelete)),
		tc.ui.DoDefault(sampleDesign),
		tc.ui.DoDefault(btnDelete),
		tc.ui.WaitUntilExists(deletionCheckText),
		tc.ui.DoDefaultUntil(btnFinalDelete, tc.ui.Gone(btnFinalDelete)),
	)(ctx)
}

// GetEditorWindowRect gets editor window's rectangular region.
// The editor window is the design editing area of TinkerCAD.
func (tc *TinkerCad) GetEditorWindowRect(ctx context.Context) (coords.Rect, error) {
	// Wait for quiescence before gets the editor window rectangular.
	if err := webutil.WaitForQuiescence(ctx, tc.conn, 15*time.Second); err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to wait TinkerCAD website for quiescence")
	}
	rect, err := tc.ui.Location(ctx, editorArea)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get the editor window location")
	}
	return *rect, nil
}

// DisableGrid disable show grid option.
func (tc *TinkerCad) DisableGrid() action.Action {
	btnEditGrid := editorAreaFinder.Name("Edit Grid").Role(role.StaticText)
	cbContainer := nodewith.HasClass("editgrid__modal__block1__showGridContainer").Role(role.GenericContainer)
	cbShowGrid := nodewith.HasClass("editor__inspector__item__checkbox").Role(role.CheckBox).Ancestor(cbContainer)
	btnUpdateGrid := editorAreaFinder.Name("Update Grid").Role(role.Button)
	return uiauto.NamedCombine("disable show grid option",
		tc.ui.DoDefault(btnEditGrid),
		tc.ui.DoDefault(cbShowGrid),
		tc.ui.DoDefault(btnUpdateGrid))
}

// AddShapeAndRotate adds primitive shape by given shape name and rotates it.
func (tc *TinkerCad) AddShapeAndRotate(shapeName string) action.Action {
	addShape := func(ctx context.Context) error {
		shapePoint, err := tc.getShapePoint(ctx, "#sidebar-item-"+shapeName)
		if err != nil {
			return errors.Wrap(err, "failed to get the shape point")
		}
		// placementPoint is the point to place a new shape.
		// Preventing shapes from overlap the Y coordinate adds a accumulated number which multiple by addShapeCount.
		// Since there's no further node information in editor window canvas,
		// so I hardcoded following placement points and the points value are decided by my test.
		// Also the value add to X or Y which divided by a hardcoded number and it allows
		// points working on differnet models.
		placementPoint := coords.Point{
			X: tc.EditorWinRect.CenterX() - int(float64(tc.EditorWinRect.Height)/3.2),
			Y: tc.EditorWinRect.CenterY() + int(float64(tc.EditorWinRect.Width)/29)*tc.addShapeCount}
		tc.addShapeCount++
		return uiauto.NamedCombine("add shape",
			tc.Visualize(ViewHome),
			// Waiting for the design being stable enough to do the next step.
			uiauto.Sleep(waitUIDuration),
			tc.pc.Drag(shapePoint, tc.pc.DragTo(placementPoint, dragDuration)),
			// Pressing F for focus shortcut on TinkerCad and this helps draging action to be more stable.
			tc.kb.TypeKeyAction(input.KEY_F),
		)(ctx)
	}
	return uiauto.NamedCombine("add and rotate the primitive shape "+shapeName,
		addShape,
		tc.rotate,
		tc.Visualize(ViewHome))
}

// RotateAll rotates all shapes at the same time.
func (tc *TinkerCad) RotateAll() action.Action {
	return uiauto.NamedCombine("rotate all shapes together",
		tc.Visualize(ViewHome),
		// Pressing ctrl+A for select all shortcut on TinkerCad.
		tc.kb.AccelAction("ctrl+A"),
		// Pressing F for focus shortcut on TinkerCad and this helps draging action to be more stable.
		tc.kb.TypeKeyAction(input.KEY_F),
		tc.rotate,
		tc.Visualize(ViewHome))
}

// RotateViewCube rotates the design with viewcube.
func (tc *TinkerCad) RotateViewCube() action.Action {
	return func(ctx context.Context) error {
		viewCube := editorAreaFinder.HasClass("hud-element").Role(role.GenericContainer)
		viewCubeRect, err := tc.ui.Location(ctx, viewCube)
		if err != nil {
			return err
		}
		// Since there's no further node information in editor window canvas,
		// so I hardcoded following drag points and the points value are decided by my test.
		// Also the value add to X or Y which divided by a hardcoded number and it allows
		// points working on differnet models.
		aroundPoint := coords.Point{
			X: viewCubeRect.CenterX() + int(float64(tc.EditorWinRect.Height)/4.5),
			Y: viewCubeRect.CenterY()}
		topPoint := coords.Point{
			X: viewCubeRect.CenterX(),
			Y: viewCubeRect.CenterY() + tc.EditorWinRect.Width/64}
		botPoint := coords.Point{
			X: viewCubeRect.CenterX(),
			Y: viewCubeRect.CenterY() - int(float64(tc.EditorWinRect.Width)/27.4)}
		return uiauto.NamedCombine("rotate the design with viewcube",
			tc.Visualize(ViewHome),
			// Waiting for the design being stable enough to do the next step.
			uiauto.Sleep(waitUIDuration),
			tc.pc.Drag(viewCubeRect.CenterPoint(), tc.pc.DragTo(aroundPoint, dragDuration)),
			tc.pc.Drag(viewCubeRect.CenterPoint(), tc.pc.DragTo(topPoint, dragDuration)),
			tc.pc.Drag(viewCubeRect.CenterPoint(), tc.pc.DragTo(botPoint, dragDuration)),
		)(ctx)
	}
}

// Visualize visualizes the design in given view mode.
func (tc *TinkerCad) Visualize(view ViewMode) action.Action {
	return tc.ui.DoDefault(view)
}

// ExportAndVerify exports the design and verifies successfully exported.
func (tc *TinkerCad) ExportAndVerify(ctx context.Context, downloadsPath, sampleDesignName string) action.Action {
	verifyExportFile := func(ctx context.Context) error {
		files, err := filesapp.Launch(ctx, tc.tconn)
		if err != nil {
			return errors.Wrap(err, "failed to launch the Files App")
		}
		defer files.Close(ctx)
		if err := files.OpenDownloads()(ctx); err != nil {
			return errors.Wrap(err, "failed to open Downloads folder in files app")
		}
		exportFileFinder := nodewith.NameContaining(sampleDesignName).HasClass("table-row").Role(role.ListBoxOption)
		if err := tc.ui.WaitUntilExists(exportFileFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to find export file in Downloads folder")
		}
		exportFileName := "Copy of " + sampleDesignName + ".zip"
		tc.exportFilePath = filepath.Join(downloadsPath, exportFileName)
		return nil
	}
	btnExport := editorAreaFinder.Name("Export").Role(role.StaticText)
	popUpFinder := nodewith.Ancestor(nodewith.HasClass("editor__modal__tab__content").Role(role.GenericContainer))
	btnOBJ := popUpFinder.Name(".OBJ").Role(role.StaticText)
	return uiauto.NamedCombine("export the design",
		tc.ui.DoDefaultUntil(btnExport, tc.ui.Exists(btnOBJ)),
		tc.ui.DoDefault(btnOBJ),
		verifyExportFile)
}

// Close closes the TinkerCAD website.
func (tc *TinkerCad) Close(ctx context.Context) {
	tc.pc.Close()
	if tc.conn == nil {
		return
	}
	tc.conn.CloseTarget(ctx)
	tc.conn.Close()
}

// getShapePoint gets primitive shape point with query selector.
func (tc *TinkerCad) getShapePoint(ctx context.Context, shapeSelector string) (coords.Point, error) {
	getElementBounds := func(selector string) (coords.Rect, error) {
		var eleBounds coords.Rect
		if err := tc.conn.Eval(ctx, fmt.Sprintf(
			`(function() {
				  var b = document.querySelector(%q).getBoundingClientRect();
					return {
						'left': Math.round(b.left),
						'top': Math.round(b.top),
						'width': Math.round(b.width),
						'height': Math.round(b.height),
					};
				})()`,
			selector), &eleBounds); err != nil {
			return coords.Rect{}, errors.Wrapf(err, "failed to get bounds for selector %q", shapeSelector)
		}
		return eleBounds, nil
	}
	var bounds coords.Rect
	if err := testing.Poll(ctx, func(ctx context.Context) (err error) {
		if bounds, err = getElementBounds(shapeSelector); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return coords.Point{}, err
	}
	return bounds.BottomCenter(), nil
}

// rotate gets location by uidetection and rotates the icon.
func (tc *TinkerCad) rotate(ctx context.Context) error {
	rotateIcon := uidetection.CustomIcon(tc.rotateIconPath, uidetection.MinConfidence(0.6))
	rotateRect, err := tc.ud.Location(ctx, rotateIcon)
	if err != nil {
		// Sometimes uidetection failed to do stable screenshot, retry it with immediate screenshot strategy.
		immediateOps := tc.ud.WithOptions(uidetection.Retries(3)).WithScreenshotStrategy(uidetection.ImmediateScreenshot).WithTimeout(time.Minute)
		rotateRect, err = immediateOps.Location(ctx, rotateIcon)
		if err != nil {
			return errors.Wrap(err, "failed to get icon location")
		}
	}
	return tc.pc.Drag(rotateRect.CenterPoint(), tc.pc.DragTo(tc.EditorWinRect.CenterPoint(), dragDuration))(ctx)
}
