// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googleapps

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// NewGoogleSlides create a new google slide.
func NewGoogleSlides(cr *chrome.Chrome, tconn *chrome.TestConn, newWindow bool) action.Action {
	const newSlidesURL = "https://slides.new"
	ui := uiauto.New(tconn)
	var opts []cdputil.CreateTargetOption
	if newWindow {
		opts = append(opts, cdputil.WithNewWindow())
	}
	filmstripView := nodewith.Name("Filmstrip view").Role(role.TabPanel)
	gotIt := nodewith.Name("Got it").First()
	return uiauto.Combine("create google slide",
		func(ctx context.Context) error {
			testing.ContextLog(ctx, "Start to create google slide")
			conn, err := cr.NewConn(ctx, newSlidesURL, opts...)
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
				return errors.Wrap(err, "failed to wait for page to finish loading")
			}
			return nil
		},
		ui.WithTimeout(time.Minute).WaitUntilExists(filmstripView),
		ui.IfSuccessThen(ui.Exists(gotIt), ui.LeftClick(gotIt)),
	)
}

// NewSlide create a new slide, edit title and content.
func NewSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, content, pageNumber string) action.Action {
	ui := uiauto.New(tconn)
	newSlide := nodewith.Name("New slide (Ctrl+M)").Role(role.Button)
	titleNode := nodewith.Name("title").First()
	pageNumberNode := nodewith.Name(pageNumber).First()
	textNode := nodewith.Name("text").First()
	documentSavedState := nodewith.NameContaining("Document status: Saved to Drive").Role(role.Button)
	testing.ContextLog(ctx, "create a new slide and edit content, page number is ", pageNumber)
	return uiauto.Combine("create a new slide and edit content",
		ui.WaitUntilExists(newSlide),
		ui.WithTimeout(time.Minute).LeftClickUntil(newSlide, ui.WithTimeout(25*time.Second).WaitUntilExists(pageNumberNode)),
		ui.WaitUntilExists(titleNode),
		ui.DoubleClick(titleNode),
		ui.Sleep(time.Second),
		kb.TypeAction(title),
		ui.WaitUntilExists(textNode),
		ui.DoubleClick(textNode),
		ui.Sleep(time.Second),
		kb.TypeAction(content),
		ui.WaitUntilExists(documentSavedState),
	)
}

// RenameSlide renames google slide.
func RenameSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title string) action.Action {
	ui := uiauto.New(tconn)
	slideWebArea := nodewith.NameContaining("Google Slides").Role(role.RootWebArea)
	documentSavedState := nodewith.NameContaining("Document status: Saved to Drive").Role(role.Button)
	renameTextbox := nodewith.Name("Rename").ClassName("docs-title-input").Ancestor(slideWebArea).Editable().Focusable()
	testing.ContextLog(ctx, "Start to rename slide")
	return ui.Retry(5, uiauto.Combine("rename slide",
		ui.WaitUntilExists(slideWebArea),
		ui.LeftClickUntil(renameTextbox, ui.WithTimeout(5*time.Second).WaitUntilExists(renameTextbox.State("focused", true))),
		kb.AccelAction("Ctrl+A"),
		kb.TypeAction(title),
		waitForFieldTextToBe(tconn, renameTextbox, title),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(documentSavedState),
	))
}

// PresentSlide presents google slide.
func PresentSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slideCount int) action.Action {
	ui := uiauto.New(tconn)
	presentationOptionsButton := nodewith.Name("Presentation options").First()
	presentFromBeginningButton := nodewith.Name("Present from beginning").First()
	present := nodewith.Name("Present").First()
	testing.ContextLog(ctx, "Start present slide")
	return uiauto.Combine("present Slide",
		ui.WaitUntilExists(presentationOptionsButton),
		ui.LeftClickUntil(presentationOptionsButton, ui.WithTimeout(5*time.Second).WaitUntilExists(presentFromBeginningButton)),
		ui.LeftClick(presentFromBeginningButton),
		ui.WithTimeout(40*time.Second).WaitUntilGone(presentationOptionsButton),
		func(ctx context.Context) error {
			testing.ContextLog(ctx, "Switch slides")
			for i := 0; i < slideCount; i++ {
				if err := uiauto.Combine("present Slide",
					kb.AccelAction("Enter"), // Press enter to switch slide.
					ui.Sleep(time.Second),   // Sleep to wait for slide switching.
				)(ctx); err != nil {
					return errors.Wrap(err, "failed to switch slide")
				}
			}
			return nil
		},
		kb.AccelAction("Esc"), //Press Esc to leave presentation mode
		// Some of DUT models with poor performance need to wait a long time to leave presentation mode.
		ui.WithTimeout(50*time.Second).WaitUntilExists(present),
	)
}

// EditSlideTitle edits google slide title and subtitle.
func EditSlideTitle(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, subtitle string) action.Action {
	ui := uiauto.New(tconn)
	titleNode := nodewith.Name("title").First()
	subtitleNode := nodewith.Name("subtitle").First()
	documentSavedState := nodewith.NameContaining("Document status: Saved to Drive").Role(role.Button)
	testing.ContextLog(ctx, "Start to edit slide title and subtitle")
	return uiauto.Combine("edit slide and subtitle",
		ui.WaitUntilExists(titleNode),
		ui.DoubleClick(titleNode),
		ui.Sleep(time.Second),
		kb.TypeAction(title),
		ui.WaitUntilExists(subtitleNode),
		ui.DoubleClick(subtitleNode),
		ui.Sleep(time.Second),
		kb.TypeAction(subtitle),
		ui.WaitUntilExists(documentSavedState),
	)
}

// EditSlide edits google slide.
func EditSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, text, expectedText string) action.Action {
	ui := uiauto.New(tconn)
	documentSavedState := nodewith.NameContaining("Document status: Saved to Drive").Role(role.Button)
	testing.ContextLog(ctx, "Start to edit slide")
	return uiauto.Combine("edit slide",
		func(ctx context.Context) error {
			nodes := nodewith.Name(text)
			nodesInfo, err := ui.NodesInfo(ctx, nodes)
			if err != nil {
				return errors.Wrap(err, "failed to get nodes info")
			}
			return mouse.DoubleClick(tconn, nodesInfo[len(nodesInfo)-1].Location.CenterPoint(), 500*time.Millisecond)(ctx)
		},
		kb.TypeAction(expectedText),
		kb.AccelAction("Esc"),
		kb.AccelAction("Esc"),
		ui.WaitUntilExists(documentSavedState),
	)
}

// DeleteSlide deletes google slide.
func DeleteSlide(ctx context.Context, tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	slideWebArea := nodewith.NameContaining("Google Slides").Role(role.RootWebArea)
	slideHomeWebArea := nodewith.Name("Google Slides").Role(role.RootWebArea)
	application := nodewith.Role(role.Application).Ancestor(slideWebArea) // Google Slide appliction node.
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToSlidesHome := nodewith.Name("Go to Slides home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	testing.ContextLog(ctx, "Start to delete slide")
	return uiauto.Combine("delete slide",
		cuj.ExpandMenu(tconn, fileButton, menu, 482),
		ui.LeftClick(moveToTrash),
		ui.LeftClick(goToSlidesHome),
		// When leaving the edit slide, sometimes the "Leave Site?" dialog box will pop up.
		// If it appears, click the leave button.
		ui.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(leaveButton), ui.LeftClick(leaveButton)),
		ui.WithTimeout(time.Minute).WaitUntilExists(slideHomeWebArea),
	)
}
