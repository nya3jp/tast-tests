// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
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

var slideWebArea = nodewith.NameContaining(slideName).Role(role.RootWebArea)

// NewGoogleSlides returns an action that creates a new google slides from web.
func NewGoogleSlides(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser, uiHandler cuj.UIActionHandler, newWindow bool) error {
	ui := uiauto.New(tconn)
	var opts []browser.CreateTargetOption
	if newWindow {
		opts = append(opts, browser.WithNewWindow())
	}
	testing.ContextLog(ctx, "Start to create google slide")
	conn, err := uiHandler.NewChromeTab(ctx, br, cuj.NewGoogleSlidesURL, newWindow)
	if err != nil {
		return errors.Wrap(err, "failed to open the google document")
	}
	defer conn.Close()
	if err := webutil.WaitForQuiescence(ctx, conn, longUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for page to finish loading")
	}
	filmstripView := nodewith.Name("Filmstrip view").Role(role.TabPanel)
	gotIt := nodewith.Name("Got it").First()
	return uiauto.Combine("confirm to enter Google Slides",
		ui.WithTimeout(longUITimeout).WaitUntilExists(filmstripView),
		uiauto.IfSuccessThen(ui.Exists(gotIt), ui.DoDefault(gotIt)),
	)(ctx)
}

// NewSlide returns an action that creates a new slide, edits its title and content.
func NewSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, content, pageNumber string) action.Action {
	ui := uiauto.New(tconn)
	newSlide := nodewith.Name("New slide (Ctrl+M)").Role(role.Button)
	titleNode := nodewith.Name("title").First()
	pageNumberNode := nodewith.Name(pageNumber).First()
	textNode := nodewith.Name("text").First()
	return uiauto.NamedCombine(fmt.Sprintf("to create a new slide with page number %s and edit its content", pageNumber),
		ui.WaitUntilExists(newSlide),
		ui.WithTimeout(longUITimeout).DoDefaultUntil(newSlide, ui.WithTimeout(25*time.Second).WaitUntilExists(pageNumberNode)),
		ui.WaitUntilExists(titleNode),
		ui.DoubleClick(titleNode),
		uiauto.Sleep(time.Second),
		kb.TypeAction(title),
		ui.WaitUntilExists(textNode),
		ui.DoubleClick(textNode),
		uiauto.Sleep(time.Second),
		kb.TypeAction(content),
		waitForSlideSaved(tconn),
	)
}

// RenameSlide returns an action that renames google slide.
func RenameSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title string) action.Action {
	ui := uiauto.New(tconn)
	renameTextbox := nodewith.Name("Rename").ClassName("docs-title-input").Ancestor(slideWebArea).Editable().Focusable()
	return uiauto.NamedAction("rename the slide",
		ui.Retry(5, uiauto.Combine("rename slide",
			ui.WaitUntilExists(slideWebArea),
			showTheSlideMenu(tconn),
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
	presentationOptionsButton := nodewith.Name("Presentation options").First()
	// There are two versions of ui to present slide.
	presentFromBeginningButton := nodewith.NameRegex(regexp.MustCompile("(Present|Start) from beginning.*")).Role(role.MenuItem).First()
	menuBar := nodewith.Name("Menu bar").Role(role.Banner).Ancestor(slideWebArea).First()
	return uiauto.NamedCombine("present slide",
		showTheSlideMenu(tconn),
		ui.WaitUntilExists(presentationOptionsButton),
		ui.DoDefaultUntil(presentationOptionsButton, ui.WithTimeout(5*time.Second).WaitUntilExists(presentFromBeginningButton)),
		ui.DoDefault(presentFromBeginningButton),
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
		ui.WithTimeout(longUITimeout).WaitUntilExists(menuBar),
	)
}

// EditSlideTitle returns an action that edits google slide title and subtitle.
func EditSlideTitle(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, title, subtitle string) action.Action {
	ui := uiauto.New(tconn)
	titleNode := nodewith.Name("title").First()
	subtitleNode := nodewith.Name("subtitle").First()
	return uiauto.NamedCombine("edit slide title and subtitle",
		ui.WaitUntilExists(titleNode),
		ui.DoubleClick(titleNode),
		uiauto.Sleep(time.Second),
		kb.TypeAction(title),
		ui.WaitUntilExists(subtitleNode),
		ui.DoubleClick(subtitleNode),
		uiauto.Sleep(time.Second),
		kb.TypeAction(subtitle),
		waitForSlideSaved(tconn),
	)
}

// EditSlide returns an action that edits google slide.
func EditSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, text, expectedText string) action.Action {
	ui := uiauto.New(tconn)
	return uiauto.NamedCombine("edit slide",
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
	)
}

// DeleteSlide returns an action that deletes google slide.
func DeleteSlide(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	slideHomeWebArea := nodewith.Name(slideName).Role(role.RootWebArea)
	application := nodewith.Role(role.Application).Ancestor(slideWebArea) // Google Slide appliction node.
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToSlidesHome := nodewith.Name("Go to Slides home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	return uiauto.NamedCombine("delete slide",
		showTheSlideMenu(tconn),
		cuj.ExpandMenu(tconn, fileButton, menu, 482),
		ui.DoDefault(moveToTrash),
		ui.DoDefault(goToSlidesHome),
		// When leaving the edit slide, sometimes the "Leave Site?" dialog box will pop up.
		// If it appears, click the leave button.
		uiauto.IfSuccessThen(ui.WithTimeout(10*time.Second).WaitUntilExists(leaveButton),
			ui.DoDefaultUntil(leaveButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(leaveButton))),
		ui.WithTimeout(longUITimeout).WaitUntilExists(slideHomeWebArea),
	)
}

// waitForSlideSaved waits for the slide document state to be saved.
func waitForSlideSaved(tconn *chrome.TestConn) action.Action {
	return waitForDocumentSaved(tconn, slideName)
}

// showTheSlideMenu shows the hidden Slide menu.
func showTheSlideMenu(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	showTheMenusButton := nodewith.NameContaining("Show the menus").Role(role.Button)
	hideTheMenusButton := nodewith.NameContaining("Hide the menus").Role(role.Button)
	return uiauto.IfSuccessThen(ui.Exists(showTheMenusButton),
		ui.LeftClickUntil(showTheMenusButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(hideTheMenusButton)))
}
