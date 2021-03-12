// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

		googleSlides := nodewith.Name("Google Slides").Role(role.MenuItem)
		if err := uiauto.Combine("create new slide",
			ui.LeftClick(nodewith.Name("New").First()),
			ui.WithInterval(500*time.Millisecond).LeftClickUntil(googleSlides, ui.Gone(googleSlides)),
		)(ctx); err != nil {
			return err
		}
		gotIt := nodewith.Name("Got it").First()
		if err := ui.WithInterval(2 * time.Second).Exists(gotIt)(ctx); err == nil {
			if err := ui.LeftClick(gotIt)(ctx); err != nil {
				return err
			}
		}
		if err := uiauto.Combine("edit slide title and subtitle",
			ui.WithInterval(2*time.Second).DoubleClick(nodewith.Name("title").First()),
			kb.TypeAction(title),
			ui.WithInterval(500*time.Millisecond).DoubleClick(nodewith.Name("subtitle").First()),
			kb.TypeAction(subtitle))(ctx); err != nil {
			return err
		}
		return nil
	}

	newSlide := func(title, content string) error {
		if err := uiauto.Combine("new slide",
			ui.WithInterval(500*time.Millisecond).RightClick(nodewith.Name("Filmstrip view").Role(role.TabPanel)),
			ui.WithInterval(500*time.Millisecond).LeftClick(nodewith.Name("New slide Ctrl+M")))(ctx); err != nil {
			return err
		}
		if err := uiauto.Combine("edit slide title and content",
			ui.WithInterval(500*time.Millisecond).DoubleClick(nodewith.Name("title").First()),
			kb.TypeAction(title),
			ui.WithInterval(500*time.Millisecond).DoubleClick(nodewith.Name("text").First()),
			kb.TypeAction(content))(ctx); err != nil {
			return err
		}
		return nil
	}
	if err := createFile(ctx); err != nil {
		return err
	}

	const slideCount = 5
	for i := 0; i < slideCount; i++ {
		title := fmt.Sprintf(slideTitle, i+1)
		content := fmt.Sprintf(slideContent, i+1)
		if err := newSlide(title, content); err != nil {
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
		ui.LeftClickUntil(presentationOptionsButton, ui.Exists(presentFromBeginningButton)),
		ui.WaitUntilExists(presentFromBeginningButton),
		ui.LeftClickUntil(presentFromBeginningButton, ui.Gone(presentFromBeginningButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to present slide")
	}

	testing.ContextLog(ctx, "Switch slides")
	for i := 0; i < 6; i++ {
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, `failed to type enter key to switch slide`)
		}
		if err := testing.Sleep(ctx, time.Second*1); err != nil {
			return errors.Wrap(err, "failed to sleep for wait slide switching")
		}
	}

	testing.ContextLog(ctx, "Leave presentation mode")
	if err := kb.Accel(ctx, "Esc"); err != nil {
		return errors.Wrap(err, `failed to type esc to leave presentation mode`)
	}

	present := nodewith.Name("Present").First()
	if err := ui.WaitUntilExists(present)(ctx); err != nil {
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
	if err := uiauto.Combine("Edit slide",
		mouse.DoubleClick(tconn, nodesInfo[len(nodesInfo)-1].Location.CenterPoint(), 500*time.Millisecond),
		kb.TypeAction(text),
		kb.AccelAction("Esc"),
		kb.AccelAction("Esc"),
	)(ctx); err != nil {
		return err
	}

	return nil
}
