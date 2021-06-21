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
		conn, err := cr.NewConn(ctx, "https://drive.google.com", opts...)
		if err != nil {
			return err
		}
		defer conn.Close()

		newButton := nodewith.Name("New").First()
		googleSlides := nodewith.Name("Google Slides").Role(role.MenuItem)
		filmstripView := nodewith.Name("Filmstrip view").Role(role.TabPanel)
		titleNode := nodewith.Name("title").First()
		subtitleNode := nodewith.Name("subtitle").First()

		checkOKButton := func(ctx context.Context) error {
			okButton := nodewith.Name("OK").Role(role.Button)
			if err := ui.Exists(okButton); err != nil {
				// OK button doesn't exist. Just return.
				return nil
			}
			if err := ui.ImmediateLeftClick(okButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to immediately click OK button")
			}
			testing.ContextLog(ctx, "Clicked ok in the first time")
			return nil
		}

		if err := uiauto.Combine("create new slide",
			ui.WaitUntilExists(newButton),
			checkOKButton,
			// Use LeftClickUntil to make sure Google Slides menu is launched.
			ui.LeftClickUntil(newButton, ui.WithTimeout(3*time.Second).WaitUntilExists(googleSlides)),
			// The "New" menu will expand to its full size with animation. Low end DUTs will see
			// lagging for this animation. Use a longer interval to wait for the "Google Slides"
			// option to be stable to accomodate UI lagging. Otherwise it might click in the middle of
			// the animation on wrong coordinates.
			ui.WithInterval(time.Second).LeftClick(googleSlides),
			// Some DUT models with poor performance need to wait a long time.
			ui.WithTimeout(2*time.Minute).WaitUntilExists(filmstripView),
		)(ctx); err != nil {
			return err
		}
		if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for page to finish loading")
		}

		return uiauto.Combine("edit slide title and subtitle",
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

func presentSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	ui := uiauto.New(tconn)
	presentationOptionsButton := nodewith.Name("Presentation options").First()
	presentFromBeginningButton := nodewith.Name("Present from beginning").First()
	if err := uiauto.Combine("Present Slide",
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

func editSlide(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	const text = "This_is_CUJ_testing_slide_page"
	ui := uiauto.New(tconn)
	nodes := nodewith.Name(text)
	nodesInfo, err := ui.NodesInfo(ctx, nodes)
	if err != nil {
		return errors.Wrap(err, "failed to get nodes info")
	}
	return uiauto.Combine("Edit slide",
		mouse.DoubleClick(tconn, nodesInfo[len(nodesInfo)-1].Location.CenterPoint(), 500*time.Millisecond),
		kb.TypeAction(text),
		kb.AccelAction("Esc"),
		kb.AccelAction("Esc"),
	)(ctx)
}
