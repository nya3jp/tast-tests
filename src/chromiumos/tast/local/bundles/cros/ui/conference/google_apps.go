// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type googleApplication string

const (
	googleSlides googleApplication = "Google Slides"
	googleDocs   googleApplication = "Google Docs"
)

const (
	testTitle    = "CUJ_TEST"
	slideTabName = testTitle + " - Google Slides"
	docTabName   = testTitle + " - Google Docs"
	docParagraph = "The Little Prince's story follows a young prince who visits various planets in space, " +
		"including Earth, and addresses themes of loneliness, friendship, love, and loss. "
)

// presentApps creates Google Slides and Google Docs, shares screen and presents
// the specified application to the conference.
func presentApps(ctx context.Context, tconn *chrome.TestConn, tsAction cuj.UIActionHandler, cr *chrome.Chrome,
	shareScreen, stopPresenting action.Action, application googleApplication, outDir string, extendedDisplay bool) (err error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	var presentApplication func(ctx context.Context) error
	switch application {
	case googleSlides:
		presentApplication = func(ctx context.Context) error {
			return uiauto.Combine("present slide",
				presentSlide(tconn, kb),
				editSlide(tconn, kb),
			)(ctx)
		}
	case googleDocs:
		presentApplication = func(ctx context.Context) error {
			return editDoc(tconn, kb, docParagraph)(ctx)
		}
	}
	switchToTab := func(tabName string) action.Action {
		return func(ctx context.Context) error {
			testing.ContextLog(ctx, "Switch to ", tabName)
			if extendedDisplay {
				return tsAction.SwitchToAppWindowByName("Chrome", tabName)(ctx)
			}
			return tsAction.SwitchToChromeTabByName(tabName)(ctx)
		}
	}
	// slideCleanup switches to the slide page and deletes it.
	slideCleanup := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switch to the slide page and delete it")
		return uiauto.Combine("switch to the slide page and delete it",
			switchToTab(slideTabName),
			deleteSlide(tconn),
		)(ctx)
	}

	// docCleanup switches to the document page and deletes it.
	docCleanup := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switch to the document page and delete it")
		return uiauto.Combine("switch to the document page and delete it",
			switchToTab(docTabName),
			deleteDoc(tconn),
		)(ctx)
	}
	// Shorten the context to cleanup document.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Create a new Google Slide")
	if err := newGoogleSlides(ctx, cr, testTitle, extendedDisplay); err != nil {
		return err
	}
	// Delete slide after presenting.
	defer func() {
		if err := slideCleanup(cleanUpCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
		}
	}()

	testing.ContextLog(ctx, "Create a new Google Document")
	if err := newGoogleDocs(ctx, cr, testTitle, extendedDisplay); err != nil {
		return err
	}
	// Delete document after presenting.
	defer func() {
		// If presenting fails, dump the last screen before deleting the document.
		faillog.DumpUITreeWithScreenshotOnError(ctx, filepath.Join(outDir, "service"), func() bool { return err != nil }, cr, "ui_dump_last")
		if err := docCleanup(cleanUpCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the document: ", err)
		}
	}()

	testing.ContextLog(ctx, "Share screen and present ", application)
	return uiauto.Combine("share screen and present "+string(application),
		shareScreen,
		presentApplication,
		stopPresenting,
	)(ctx)
}
