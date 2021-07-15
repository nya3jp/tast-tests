// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
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

func newGoogleSlides(ctx context.Context, cr *chrome.Chrome, newWindow bool) error {
	const (
		newSlidesURL = "https://slides.new"
		title        = "CUJ Testing"
		subtitle     = "For testing only"
		slideTitle   = "CUJ_slide_title_page %d"
		slideContent = "This_is_CUJ_testing_slide_page %d"
	)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	defer tconn.Close()
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()

	createFile := func(ctx context.Context) error {
		var opts []cdputil.CreateTargetOption
		if newWindow {
			opts = append(opts, cdputil.WithNewWindow())
		}
		conn, err := cr.NewConn(ctx, newSlidesURL, opts...)
		if err != nil {
			return err
		}
		defer conn.Close()
		if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to finish loading")
		}

		filmstripView := nodewith.Name("Filmstrip view").Role(role.TabPanel)
		titleNode := nodewith.Name("title").First()
		subtitleNode := nodewith.Name("subtitle").First()

		return uiauto.Combine("edit slide title and subtitle",
			ui.WithTimeout(time.Minute).WaitUntilExists(filmstripView),
			ui.WaitUntilExists(titleNode),
			ui.DoubleClick(titleNode),
			ui.Sleep(time.Second),
			kb.TypeAction(title),
			ui.WaitUntilExists(subtitleNode),
			ui.DoubleClick(subtitleNode),
			ui.Sleep(time.Second),
			kb.TypeAction(subtitle))(ctx)
	}

	newSlide := func(title, content, pageNumber string) error {
		testing.ContextLog(ctx, "Create new slide and edit content, page number is ", pageNumber)
		newSlide := nodewith.Name("New slide (Ctrl+M)").Role(role.Button)
		titleNode := nodewith.Name("title").First()
		pageNumberNode := nodewith.Name(pageNumber).First()
		if err := uiauto.Combine("new slide",
			ui.WaitUntilExists(newSlide),
			ui.WithTimeout(time.Minute).LeftClickUntil(newSlide, ui.WithTimeout(25*time.Second).WaitUntilExists(pageNumberNode)),
		)(ctx); err != nil {
			return err
		}

		textNode := nodewith.Name("text").First()
		return uiauto.Combine("edit slide title and content",
			ui.WaitUntilExists(titleNode),
			ui.DoubleClick(titleNode),
			ui.Sleep(time.Second),
			kb.TypeAction(title),
			ui.WaitUntilExists(textNode),
			ui.DoubleClick(textNode),
			ui.Sleep(time.Second),
			kb.TypeAction(content))(ctx)
	}
	if err := createFile(ctx); err != nil {
		return err
	}

	gotIt := nodewith.Name("Got it").First()
	if err := ui.Exists(gotIt)(ctx); err == nil {
		if err := ui.LeftClick(gotIt)(ctx); err != nil {
			return err
		}
	}

	const slideCount = 5
	for i := 0; i < slideCount; i++ {
		title := fmt.Sprintf(slideTitle, i+1)
		content := fmt.Sprintf(slideContent, i+1)
		pageNumber := strconv.Itoa(i + 2)
		if err := newSlide(title, content, pageNumber); err != nil {
			return err
		}
	}

	return nil
}

func presentSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) action.Action {
	ui := uiauto.New(tconn)
	presentationOptionsButton := nodewith.Name("Presentation options").First()
	presentFromBeginningButton := nodewith.Name("Present from beginning").First()
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start present slide")
		if err := uiauto.Combine("present Slide",
			ui.WaitUntilExists(presentationOptionsButton),
			ui.LeftClickUntil(presentationOptionsButton, ui.WithTimeout(5*time.Second).WaitUntilExists(presentFromBeginningButton)),
			ui.LeftClick(presentFromBeginningButton),
			ui.WithTimeout(40*time.Second).WaitUntilGone(presentationOptionsButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to present slide")
		}

		testing.ContextLog(ctx, "Switch slides")
		for i := 0; i < 6; i++ {
			if err := kb.Accel(ctx, "Enter"); err != nil {
				return errors.Wrap(err, `failed to type enter key to switch slide`)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep for wait slide switching")
			}
		}

		testing.ContextLog(ctx, "Leave presentation mode")
		if err := kb.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, `failed to type esc to leave presentation mode`)
		}
		// Some of DUT models with poor performance need to wait a long time to leave presentation mode.
		present := nodewith.Name("Present").First()
		if err := ui.WithTimeout(50 * time.Second).WaitUntilExists(present)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait 'Present'")
		}

		return nil
	}
}

func editSlide(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) action.Action {
	const text = "This_is_CUJ_testing_slide_page"
	ui := uiauto.New(tconn)
	return func(ctx context.Context) error {
		nodes := nodewith.Name(text)
		nodesInfo, err := ui.NodesInfo(ctx, nodes)
		if err != nil {
			return errors.Wrap(err, "failed to get nodes info")
		}
		testing.ContextLog(ctx, "Start to edit slide")
		return uiauto.Combine("edit slide",
			mouse.DoubleClick(tconn, nodesInfo[len(nodesInfo)-1].Location.CenterPoint(), 500*time.Millisecond),
			kb.TypeAction(text),
			kb.AccelAction("Esc"),
			kb.AccelAction("Esc"),
		)(ctx)
	}
}

func deleteSlide(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)
	application := nodewith.Role(role.Application) // Google Slide appliction node.
	slideWebArea := nodewith.Name("Google Slides").Role(role.RootWebArea)
	fileButton := nodewith.Name("File").Role(role.MenuItem).Ancestor(application)
	menu := nodewith.Role(role.Menu).Ancestor(application)
	moveToTrash := nodewith.NameContaining("Move to trash t").Role(role.MenuItem)
	goToSlidesHome := nodewith.Name("Go to Slides home screen").Role(role.Button)
	leaveButton := nodewith.Name("Leave").Role(role.Button)
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start to delete slide")
		return uiauto.Combine("delete slide",
			expandMenu(tconn, fileButton, menu, 482),
			ui.LeftClick(moveToTrash),
			ui.LeftClick(goToSlidesHome),
			// When leaving the edit slide, sometimes the "Leave Site?" dialog box will pop up.
			// If it appears, click the leave button.
			ui.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(leaveButton), ui.LeftClick(leaveButton)),
			ui.WithTimeout(time.Minute).WaitUntilExists(slideWebArea),
		)(ctx)
	}
}
