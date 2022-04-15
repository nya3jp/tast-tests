// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googleapps

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// slideName represents the name of the Google Slides web area.
const slideName = "Google Slides"

// NewGoogleSlides returns an action that creates a new google slides from web.
func NewGoogleSlides(cs ash.ConnSource, tconn *chrome.TestConn, newWindow bool) action.Action {
	const newSlidesURL = "https://slides.new"
	ui := uiauto.New(tconn)
	var opts []browser.CreateTargetOption
	if newWindow {
		opts = append(opts, browser.WithNewWindow())
	}
	filmstripView := nodewith.Name("Filmstrip view").Role(role.TabPanel)
	gotIt := nodewith.Name("Got it").First()
	return uiauto.Combine("create google slide",
		func(ctx context.Context) error {
			testing.ContextLog(ctx, "Start to create google slide")
			conn, err := cs.NewConn(ctx, newSlidesURL, opts...)
			if err != nil {
				return err
			}
			defer conn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, longerUIWaitTime); err != nil {
				return errors.Wrap(err, "failed to wait for page to finish loading")
			}
			return nil
		},
		ui.WithTimeout(longerUIWaitTime).WaitUntilExists(filmstripView),
		uiauto.IfSuccessThen(ui.Exists(gotIt), ui.LeftClick(gotIt)),
	)
}

// NewSlide returns an action that creates a new slide, edits its title and content.
func NewSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, content, pageNumber string) action.Action {
	ui := uiauto.New(tconn)
	newSlide := nodewith.Name("New slide (Ctrl+M)").Role(role.Button)
	titleNode := nodewith.Name("title").First()
	pageNumberNode := nodewith.Name(pageNumber).First()
	textNode := nodewith.Name("text").First()
	return uiauto.NamedAction(fmt.Sprintf("to create a new slide with page number %s and edit its content", pageNumber),
		uiauto.Combine("create a new slide and edit content",
			ui.WaitUntilExists(newSlide),
			ui.WithTimeout(longerUIWaitTime).LeftClickUntil(newSlide, ui.WithTimeout(25*time.Second).WaitUntilExists(pageNumberNode)),
			ui.WaitUntilExists(titleNode),
			ui.DoubleClick(titleNode),
			uiauto.Sleep(time.Second),
			kb.TypeAction(title),
			ui.WaitUntilExists(textNode),
			ui.DoubleClick(textNode),
			uiauto.Sleep(time.Second),
			kb.TypeAction(content),
			waitForSlideSaved(tconn),
		),
	)
}

// RenameSlide returns an action that renames google slide.
func RenameSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title string) action.Action {
	ui := uiauto.New(tconn)
	slideWebArea := nodewith.NameContaining(slideName).Role(role.RootWebArea)
	renameTextbox := nodewith.Name("Rename").ClassName("docs-title-input").Ancestor(slideWebArea).Editable().Focusable()
	return uiauto.NamedAction("to rename the slide",
		ui.Retry(5, uiauto.Combine("rename slide",
			ui.WaitUntilExists(slideWebArea),
			ui.LeftClickUntil(renameTextbox, ui.WithTimeout(5*time.Second).WaitUntilExists(renameTextbox.State("focused", true))),
			kb.AccelAction("Ctrl+A"),
			kb.TypeAction(title),
			waitForFieldTextToBe(tconn, renameTextbox, title),
			kb.AccelAction("Enter"),
			waitForSlideSaved(tconn),
		)),
	)
}

// PresentSlide returns an action that presents google slide.
func PresentSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slideCount int) action.Action {
	ui := uiauto.New(tconn)
	slideWebArea := nodewith.NameContaining(slideName).Role(role.RootWebArea)
	presentationOptionsButton := nodewith.Name("Presentation options").First()
	// There are two versions of ui to present slide.
	presentFromBeginningButton := nodewith.NameRegex(regexp.MustCompile("(Present|Start) from beginning.*")).Role(role.MenuItem).First()
	menuBar := nodewith.Name("Menu bar").Role(role.Banner).Ancestor(slideWebArea).First()
	return uiauto.NamedAction("to present slide",
		uiauto.Combine("present Slide",
			ui.WaitUntilExists(presentationOptionsButton),
			ui.LeftClickUntil(presentationOptionsButton, ui.WithTimeout(5*time.Second).WaitUntilExists(presentFromBeginningButton)),
			ui.LeftClick(presentFromBeginningButton),
			ui.WithTimeout(40*time.Second).WaitUntilGone(presentationOptionsButton),
			func(ctx context.Context) error {
				testing.ContextLog(ctx, "Switch slides")
				for i := 0; i < slideCount; i++ {
					if err := uiauto.Combine("present Slide",
						kb.AccelAction("Enter"),   // Press enter to switch slide.
						uiauto.Sleep(time.Second), // Sleep to wait for slide switching.
					)(ctx); err != nil {
						return errors.Wrap(err, "failed to switch slide")
					}
				}
				return nil
			},
			kb.AccelAction("Esc"), //Press Esc to leave presentation mode
			// Some of DUT models with poor performance need to wait a long time to leave presentation mode.
			ui.WithTimeout(longerUIWaitTime).WaitUntilExists(menuBar),
		),
	)
}

// EditSlideTitle returns an action that edits google slide title and subtitle.
func EditSlideTitle(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, subtitle string) action.Action {
	ui := uiauto.New(tconn)
	titleNode := nodewith.Name("title").First()
	subtitleNode := nodewith.Name("subtitle").First()
	return uiauto.NamedAction("to edit slide title and subtitle",
		uiauto.Combine("edit slide and subtitle",
			ui.WaitUntilExists(titleNode),
			ui.DoubleClick(titleNode),
			uiauto.Sleep(time.Second),
			kb.TypeAction(title),
			ui.WaitUntilExists(subtitleNode),
			ui.DoubleClick(subtitleNode),
			uiauto.Sleep(time.Second),
			kb.TypeAction(subtitle),
			waitForSlideSaved(tconn),
		),
	)
}

// EditSlide returns an action that edits google slide.
func EditSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, text, expectedText string) action.Action {
	ui := uiauto.New(tconn)
	return uiauto.NamedAction("to edit slide",
		uiauto.Combine("edit slide",
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
			waitForSlideSaved(tconn),
		),
	)
}

// DeleteSlide returns an action that deletes google slide.
func DeleteSlide(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	slideWebArea := nodewith.NameContaining(slideName).Role(role.RootWebArea)
	slideHomeWebArea := nodewith.Name(slideName).Role(role.RootWebArea)
	application := nodewith.Role(role.Application).Ancestor(slideWebArea) // Google Slide appliction node.
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToSlidesHome := nodewith.Name("Go to Slides home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	return uiauto.NamedAction("to delete slide",
		uiauto.Combine("delete slide",
			cuj.ExpandMenu(tconn, fileButton, menu, 482),
			ui.LeftClick(moveToTrash),
			ui.LeftClick(goToSlidesHome),
			// When leaving the edit slide, sometimes the "Leave Site?" dialog box will pop up.
			// If it appears, click the leave button.
			uiauto.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(leaveButton), ui.LeftClick(leaveButton)),
			ui.WithTimeout(longerUIWaitTime).WaitUntilExists(slideHomeWebArea),
		),
	)
}

// waitForSlideSaved waits for the slide document state to be saved.
func waitForSlideSaved(tconn *chrome.TestConn) action.Action {
	return waitForDocumentSaved(tconn, slideName)
}
