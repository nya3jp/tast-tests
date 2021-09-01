// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/googleapps"
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
	testTitle        = "CUJ_TEST"
	slideTabName     = testTitle + " - Google Slides"
	slideTitle       = "CUJ_slide_title"
	slideSubTitle    = "For testing only"
	slideEditContent = "This_is_CUJ_testing_after_edit"
	slideCount       = 5
	subSlideTitle    = "CUJ_sub_slide_title"
	subSlideContent  = "This_is_CUJ_testing_sub_slide_content"
	docTabName       = testTitle + " - Google Docs"
	docParagraph     = "The Little Prince's story follows a young prince who visits various planets in space, " +
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
	ui := uiauto.New(tconn)
	var presentApplication action.Action
	switch application {
	case googleSlides:
		presentApplication = uiauto.Combine("present slide",
			googleapps.PresentSlide(ctx, tconn, kb, slideCount+1),
			googleapps.EditSlide(ctx, tconn, kb, subSlideContent, slideEditContent),
		)
	case googleDocs:
		presentApplication = googleapps.EditDoc(ctx, tconn, kb, docParagraph)
	}
	switchToTab := func(tabName string) action.Action {
		testing.ContextLog(ctx, "Switch to ", tabName)
		if extendedDisplay {
			return tsAction.SwitchToAppWindowByName("Chrome", tabName)
		}
		return tsAction.SwitchToChromeTabByName(tabName)
	}

	var renameSlideErr error
	var editSlideErr error
	// slideCleanup switches to the slide page and deletes it.
	slideCleanup := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switch to the slide page and delete it")
		return uiauto.Combine("switch to the slide page and delete it",
			ui.IfSuccessThen(func(ctx context.Context) error { return renameSlideErr }, switchToTab(string(googleSlides))),
			googleapps.DeleteSlide(ctx, tconn),
		)(ctx)
	}

	var renameDocErr error
	// docCleanup switches to the document page and deletes it.
	docCleanup := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switch to the document page and delete it")
		return uiauto.Combine("switch to the document page and delete it",
			ui.IfSuccessThen(func(ctx context.Context) error { return renameDocErr }, switchToTab(string(googleDocs))),
			googleapps.DeleteDoc(ctx, tconn),
		)(ctx)
	}
	// Shorten the context to cleanup document.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Create a new Google Slide")
	if err := googleapps.NewGoogleSlides(cr, tconn, extendedDisplay)(ctx); err != nil {
		return err
	}
	// Delete slide after presenting.
	defer func() {
		// If rename or edit slide fails, dump the last screen before deleting the slide.
		faillog.DumpUITreeWithScreenshotOnError(ctx, filepath.Join(outDir, "service"),
			func() bool { return renameSlideErr != nil || editSlideErr != nil }, cr, "ui_dump_slide")

		if err := slideCleanup(cleanUpCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
		}
	}()
	renameSlideErr = googleapps.RenameSlide(ctx, tconn, kb, testTitle)(ctx)
	if renameSlideErr != nil {
		return renameSlideErr
	}
	editSlideErr = googleapps.EditSlideTitle(ctx, tconn, kb, slideTitle, slideSubTitle)(ctx)
	if editSlideErr != nil {
		return editSlideErr
	}
	for i := 0; i < slideCount; i++ {
		title := fmt.Sprintf(subSlideTitle+" page %d", i+1)
		content := fmt.Sprintf(subSlideContent+" page %d", i+1)
		pageNumber := strconv.Itoa(i + 2)
		editSlideErr = googleapps.NewSlide(ctx, tconn, kb, title, content, pageNumber)(ctx)
		if editSlideErr != nil {
			return editSlideErr
		}
	}

	testing.ContextLog(ctx, "Create a new Google Document")
	if err := googleapps.NewGoogleDocs(cr, tconn, extendedDisplay)(ctx); err != nil {
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
	renameDocErr = googleapps.RenameDoc(ctx, tconn, kb, testTitle)(ctx)
	if renameDocErr != nil {
		return renameDocErr
	}
	testing.ContextLog(ctx, "Share screen and present ", application)
	return uiauto.Combine("share screen and present "+string(application),
		shareScreen,
		presentApplication,
		stopPresenting,
	)(ctx)
}
