// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imageeditingcuj contains imageedit CUJ test cases library.
package imageeditingcuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	dragTime       = 2 * time.Second
	shortUITimeout = 5 * time.Second
)

var (
	googlePhotosWebArea = nodewith.Name("Google Photos").Role(role.RootWebArea)
	photosWebArea       = nodewith.Name("Photos - Google Photos").Role(role.RootWebArea)
	photosWebAreaFinder = nodewith.Ancestor(photosWebArea)
	photoWebArea        = nodewith.Name("Photo - Google Photos").Role(role.RootWebArea)
	editPhotoWebArea    = nodewith.Name("Edit photo - Google Photos").Role(role.RootWebArea)
	canvas              = nodewith.Role(role.Canvas).Ancestor(editPhotoWebArea)
	editButton          = nodewith.Name("Edit").Role(role.Button).Ancestor(photoWebArea)
	doneButton          = nodewith.Name("Done").Role(role.Button).Ancestor(editPhotoWebArea)
)

// GooglePhotos holds the information used to do Google Photos testing.
type GooglePhotos struct {
	br         *browser.Browser
	tconn      *chrome.TestConn
	ui         *uiauto.Context
	shortUI    *uiauto.Context
	uiHdl      cuj.UIActionHandler
	kb         *input.KeyboardEventWriter
	conn       *chrome.Conn
	password   string
	tabletMode bool
}

// NewGooglePhotos returns the the manager of Google Photos, caller will able to control Google Photos through this object.
func NewGooglePhotos(tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, password string, tabletMode bool) *GooglePhotos {
	ui := uiauto.New(tconn)
	return &GooglePhotos{
		tconn:      tconn,
		ui:         ui,
		shortUI:    ui.WithTimeout(shortUITimeout),
		uiHdl:      uiHdl,
		kb:         kb,
		password:   password,
		tabletMode: tabletMode,
	}
}

// SetBrowser sets browser to chrome or lacros.
func (g *GooglePhotos) SetBrowser(br *browser.Browser) {
	g.br = br
}

// ClickIfExists clicks if the finder appears in shortUITimeout.
func (g *GooglePhotos) ClickIfExists(finder *nodewith.Finder) uiauto.Action {
	return uiauto.IfSuccessThen(g.shortUI.WaitUntilExists(finder), g.uiHdl.Click(finder))
}

// ClickIfExistsWithLog clicks if the finder appears in shortUITimeout and log when the finder is clicked.
func (g *GooglePhotos) ClickIfExistsWithLog(finder *nodewith.Finder, logText string) uiauto.Action {
	return uiauto.IfSuccessThen(
		g.shortUI.WaitUntilExists(finder),
		uiauto.NamedAction(logText, g.uiHdl.Click(finder)),
	)
}

// DoDefaultIfExists calls DoDefault function if the finder appears in shortUITimeout.
func (g *GooglePhotos) DoDefaultIfExists(finder *nodewith.Finder) uiauto.Action {
	return uiauto.IfSuccessThen(g.shortUI.WaitUntilExists(finder), g.ui.DoDefault(finder))
}

// Open opens the Google Photos.
func (g *GooglePhotos) Open() uiauto.Action {
	return func(ctx context.Context) (err error) {
		g.conn, err = g.br.NewConn(ctx, cuj.GooglePhotosURL)
		if err != nil {
			return errors.Wrapf(err, "failed to open URL: %s", cuj.GooglePhotosURL)
		}

		if err := cuj.MaximizeBrowserWindow(ctx, g.tconn, g.tabletMode, "Google Photos"); err != nil {
			return err
		}

		nextButton := photosWebAreaFinder.Name("Next").Role(role.Button).Focusable()
		checkIfLoggedIn := uiauto.NamedCombine("check if logged in",
			// If logged in, it might pop up the "Verify it's you" heading as a reminder.
			g.ClickIfExistsWithLog(nextButton, "skip verify dialog"),
			g.shortUI.WaitUntilExists(photosWebArea.Focusable()),
		)

		link := photosWebAreaFinder.Name("Go to Google Photos").Role(role.Link).First()
		logInAndEnterGooglePhotos := uiauto.Combine("log in and enter Google Photos",
			// If not logged in, the URL will navigate to the Google Photos homepage.
			g.ClickIfExistsWithLog(link, "click 'Go to' button"),
			g.login(),
		)

		continueButton := photosWebAreaFinder.Name("Continue").Role(role.Button)
		return uiauto.Combine("open Google Photo",
			uiauto.IfFailThen(checkIfLoggedIn, logInAndEnterGooglePhotos),
			g.ClickIfExistsWithLog(continueButton, "skip Google Photos dialog"),
		)(ctx)
	}
}

// login logs in to the google account.
func (g *GooglePhotos) login() uiauto.Action {
	confirmInput := func(finder *nodewith.Finder, input string) uiauto.Action {
		return func(ctx context.Context) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				if err := g.kb.TypeAction(input)(ctx); err != nil {
					return err
				}
				node, err := g.ui.Info(ctx, finder)
				if err != nil {
					return err
				}
				if node.Value == input {
					return nil
				}
				if err := g.kb.AccelAction("Ctrl+A")(ctx); err != nil {
					return err
				}
				return errors.Errorf("%s is incorrect: got: %v; want: %v", node.Name, node.Value, input)
			}, &testing.PollOptions{Timeout: 30 * time.Second})
		}
	}

	continueButton := nodewith.Name("Continue").Role(role.Button).Ancestor(photosWebArea)
	nextButton := nodewith.Name("Next").Role(role.Button).Ancestor(googlePhotosWebArea)
	continueButton = nodewith.Name("Continue").Role(role.Button).Ancestor(googlePhotosWebArea)
	showPasswordCheckBox := nodewith.Name("Show password").Role(role.CheckBox).Focusable()
	passwordField := nodewith.Name("Enter your password").Role(role.TextField).Editable()
	return uiauto.NamedAction("login google account",
		uiauto.IfFailThen(
			g.shortUI.WaitUntilExists(continueButton),
			uiauto.Combine("login google account",
				g.ClickIfExists(nextButton),
				g.ClickIfExists(continueButton),
				g.ui.WaitUntilExists(passwordField),
				g.ui.LeftClick(showPasswordCheckBox),
				g.ui.LeftClick(passwordField),
				confirmInput(passwordField, g.password),
				g.kb.AccelAction("Enter"),
			),
		),
	)
}

// Upload uploads the photo from the Downloads directory.
func (g *GooglePhotos) Upload(fileName string) uiauto.Action {
	uploadButton := nodewith.Name("Upload photos").Role(role.PopUpButton).Focusable()
	computerItem := nodewith.Name("Computer").Role(role.MenuItem)
	filesWebAreaFinder := nodewith.Ancestor(nodewith.Name("Files").Role(role.RootWebArea))
	downloadsItem := filesWebAreaFinder.Name("Downloads").Role(role.TreeItem)
	testImageOption := filesWebAreaFinder.NameStartingWith(fileName).Role(role.ListBoxOption).Focusable()
	openButton := nodewith.Name("Open").Role(role.Button)
	continueButton := nodewith.Name("Continue").Role(role.Button)
	closeDialogButton := nodewith.Name("Close dialog").Role(role.Button).Focusable()
	noThanksButton := nodewith.Name("No thanks").Role(role.Button).Focusable()
	return uiauto.NamedCombine("upload the photo",
		g.ui.DoDefault(uploadButton),
		g.ui.DoDefaultUntil(computerItem, g.shortUI.WaitUntilExists(downloadsItem)),
		g.ui.DoDefault(downloadsItem),
		g.uiHdl.Click(testImageOption),
		g.uiHdl.Click(openButton),
		g.DoDefaultIfExists(continueButton),
		g.DoDefaultIfExists(closeDialogButton),
		g.DoDefaultIfExists(noThanksButton),
	)
}

// AddFilters adds filters to the photo.
func (g *GooglePhotos) AddFilters() uiauto.Action {
	photoLink := photosWebAreaFinder.NameStartingWith("Photo - Square").Role(role.Link)
	filtersGroup := nodewith.Name("Filters").Role(role.RadioGroup)
	autoButton := nodewith.Name("Auto").Role(role.RadioButton).Ancestor(filtersGroup)
	autoButtonFocused := autoButton.Focused()
	blushButton := nodewith.Name("Blush").Role(role.RadioButton).Ancestor(filtersGroup)
	blushButtonFocused := blushButton.Focused()
	return uiauto.NamedCombine("add filters to photo",
		g.ui.DoDefaultUntil(photoLink, g.shortUI.WaitUntilExists(editButton)),
		g.ui.DoDefault(editButton),
		g.ui.DoDefaultUntil(autoButton, g.shortUI.WaitUntilExists(autoButtonFocused)),
		g.ui.DoDefaultUntil(blushButton, g.shortUI.WaitUntilExists(blushButtonFocused)),
		uiauto.IfSuccessThen(g.ui.Gone(doneButton), g.ui.MouseMoveTo(canvas, dragTime)),
		g.ui.DoDefault(doneButton),
	)
}

// Edit changes brightness, sharpness and color depth.
func (g *GooglePhotos) Edit() uiauto.Action {
	dragLightSlider := func(ctx context.Context) error {
		location, err := g.getSliderLocation(ctx, "Light")
		if err != nil {
			return err
		}
		startPoint := location.CenterPoint()
		x := float64(location.CenterX()) + float64(location.Width)*0.5*0.25
		y := location.CenterY()
		endPoint := coords.NewPoint(int(x), int(y))
		return mouse.Drag(g.tconn, startPoint, endPoint, dragTime)(ctx)
	}

	okButton := nodewith.Name("OK").Role(role.Button).Focusable().Ancestor(editPhotoWebArea)
	closeDialogButton := nodewith.Name("Close dialog").Role(role.Button).Focusable().Ancestor(editPhotoWebArea)
	adjustmentsTab := nodewith.Name("Basic adjustments").Role(role.Tab)
	return uiauto.NamedCombine("edit the photo",
		g.DoDefaultIfExists(okButton),
		g.DoDefaultIfExists(closeDialogButton),
		g.ui.DoDefaultUntil(editButton, g.shortUI.WaitUntilExists(adjustmentsTab)),
		g.ui.DoDefault(adjustmentsTab),
		dragLightSlider,
		g.ui.DoDefaultUntil(doneButton, g.shortUI.WaitUntilGone(doneButton)),
	)
}

// Rotate rotates the photo.
func (g *GooglePhotos) Rotate() uiauto.Action {
	closeDialogButton := nodewith.Name("Close dialog").Role(role.Button).Focusable().Ancestor(editPhotoWebArea)
	rotateButton := nodewith.Name("Crop & rotate").Role(role.Button)
	rotateLeftButton := nodewith.Name("Rotate left").Role(role.Button)
	confirmButton := nodewith.Name("Confirm crop & rotate").Role(role.Button)
	return uiauto.NamedCombine("rotate the photo",
		g.DoDefaultIfExists(closeDialogButton),
		g.ui.DoDefaultUntil(editButton, g.shortUI.WaitUntilExists(rotateButton)),
		g.ui.DoDefaultUntil(rotateButton, g.shortUI.WaitUntilExists(rotateLeftButton)),
		g.ui.DoDefault(rotateLeftButton),
		g.ui.DoDefaultUntil(confirmButton, g.shortUI.WaitUntilGone(confirmButton)),
	)
}

// ReduceColor reduces colors to convert photos to black and white.
func (g *GooglePhotos) ReduceColor() uiauto.Action {
	return func(ctx context.Context) error {
		adjustmentsTab := nodewith.Name("Basic adjustments").Role(role.Tab)
		if err := g.ui.DoDefault(adjustmentsTab)(ctx); err != nil {
			return err
		}
		location, err := g.getSliderLocation(ctx, "Color")
		if err != nil {
			return err
		}

		initialPoint := location.CenterPoint()
		blackLocationPoint := location.LeftCenter()
		whiteLocationPoint := location.RightCenter()
		return uiauto.NamedCombine("convert to black and white",
			mouse.Drag(g.tconn, initialPoint, blackLocationPoint, dragTime),
			uiauto.Sleep(2*time.Second), // Given the time to make change.
			mouse.Drag(g.tconn, blackLocationPoint, whiteLocationPoint, dragTime*2),
			uiauto.Sleep(2*time.Second), // Given the time to make change.
			g.ui.DoDefaultUntil(doneButton, g.shortUI.WaitUntilGone(doneButton)),
		)(ctx)
	}
}

// Crop crops the photo.
func (g *GooglePhotos) Crop() uiauto.Action {
	dragCropFrame := func(ctx context.Context) error {
		location, err := g.getSliderLocation(ctx, "Crop frame position")
		if err != nil {
			return err
		}
		startPoint := location.TopRight()
		endPoint := location.CenterPoint()
		return mouse.Drag(g.tconn, startPoint, endPoint, dragTime)(ctx)
	}
	cropButton := nodewith.Name("Crop & rotate").Role(role.Button)
	confirmButton := nodewith.Name("Confirm crop & rotate").Role(role.Button)
	return uiauto.NamedCombine("crop the photo",
		g.ui.DoDefault(editButton),
		g.ui.DoDefault(cropButton),
		dragCropFrame,
		g.uiHdl.Click(confirmButton),
	)
}

// UndoEdit undo all edits.
func (g *GooglePhotos) UndoEdit() uiauto.Action {
	undo := nodewith.Name("Undo edits").Role(role.Button)
	return uiauto.NamedCombine("undo all the edits",
		g.ui.MouseMoveTo(canvas, dragTime),
		g.ui.DoDefault(undo),
		g.ui.DoDefault(doneButton),
	)
}

// CleanUp deletes the photo uploaded at the beginning of the test case.
func (g *GooglePhotos) CleanUp() uiauto.Action {
	enterEditPage := func(ctx context.Context) error {
		deleteButton := nodewith.Name("Delete").Role(role.Button).Ancestor(editPhotoWebArea)
		if err := g.shortUI.WaitUntilExists(deleteButton)(ctx); err == nil {
			return nil
		}
		if err := g.shortUI.WaitUntilExists(editPhotoWebArea.Focused())(ctx); err == nil {
			return uiauto.NamedCombine("leave the edit mode",
				g.ui.MouseMoveTo(canvas, dragTime),
				g.ui.DoDefault(doneButton),
			)(ctx)
		}
		photoLink := nodewith.NameStartingWith("Photo - Square").Role(role.Link).Ancestor(photosWebArea)
		return uiauto.NamedAction("leave main page", g.uiHdl.Click(photoLink))(ctx)
	}
	return uiauto.NamedCombine("delete the photo",
		enterEditPage,
		g.deletePhoto(),
	)
}

// Close closes the connection.
func (g *GooglePhotos) Close(ctx context.Context) {
	if g.conn == nil {
		return
	}
	g.conn.CloseTarget(ctx)
	g.conn.Close()
}

func (g *GooglePhotos) getSliderLocation(ctx context.Context, name string) (coords.Rect, error) {
	slider := nodewith.Name(name).Role(role.Slider).Ancestor(editPhotoWebArea)
	node, err := g.ui.Info(ctx, slider)
	if err != nil {
		return coords.Rect{}, err
	}
	return node.Location, nil
}

func (g *GooglePhotos) deletePhoto() uiauto.Action {
	deleteButton := nodewith.Name("Delete").Role(role.Button).Ancestor(photoWebArea)
	alertDialog := nodewith.Role(role.AlertDialog).Ancestor(photoWebArea)
	gotItButton := nodewith.Name("Got it").Role(role.Button).Ancestor(alertDialog)
	moveToTrashButton := nodewith.Name("Move to trash").Role(role.Button)
	return uiauto.NamedCombine("delete the photo",
		g.uiHdl.Click(deleteButton),
		g.ClickIfExists(gotItButton),
		g.uiHdl.Click(moveToTrashButton),
	)
}
