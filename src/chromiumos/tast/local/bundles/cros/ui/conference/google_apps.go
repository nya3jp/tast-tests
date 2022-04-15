// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/googleapps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
	slideCount       = 2
	subSlideTitle    = "CUJ_sub_slide_title"
	subSlideContent  = "This_is_CUJ_testing_sub_slide_content"
	docTabName       = testTitle + " - Google Docs"
	docParagraph     = "The Little Prince's story follows a young prince who visits various planets in space, " +
		"including Earth, and addresses themes of loneliness, friendship, love, and loss. "
)

// presentApps creates Google Slides and Google Docs, shares screen and presents
// the specified application to the conference.
func presentApps(ctx context.Context, tconn *chrome.TestConn, uiHandler cuj.UIActionHandler, cr *chrome.Chrome, cs ash.ConnSource,
	shareScreen, stopPresenting action.Action, application googleApplication, outDir string, extendedDisplay bool) (err error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()
	ui := uiauto.New(tconn)
	var presentApplication action.Action
	switchToTab := func(tabName string) action.Action {
		act := uiHandler.SwitchToChromeTabByName(tabName)
		if extendedDisplay {
			act = uiHandler.SwitchToAppWindowByName("Chrome", tabName)
		}
		return uiauto.NamedAction("switch tab to "+tabName, act)
	}
	switchTabIfNeeded := func(ctx context.Context) error {
		// Some DUTs will switch to application tab when sharing screen.
		// If there is no auto-switch, switch the tab to the application page.
		appName := string(application)
		appWebArea := nodewith.NameContaining(appName).Role(role.RootWebArea)
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(appWebArea)(ctx); err == nil {
			testing.ContextLogf(ctx, "Already on the %s app page", application)
		}
		// Check whether current window is expected or not.
		// If it stays on the expected window, there is no need to switch tab.
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.IsActive && w.IsFrameVisible
		})
		if err != nil {
			return errors.Wrap(err, "failed to get current active window")
		}
		if strings.Contains(w.Title, appName) {
			testing.ContextLogf(ctx, "Current chrome window is %s, no need to switch tab", w.Title)
			return nil
		}

		return switchToTab(appName)(ctx)
	}
	switch application {
	case googleSlides:
		presentApplication = uiauto.Combine("present slide",
			googleapps.PresentSlide(tconn, kb, slideCount+1),
			googleapps.EditSlide(tconn, kb, subSlideContent, slideEditContent),
		)
	case googleDocs:
		presentApplication = googleapps.EditDoc(tconn, kb, docParagraph)
	}

	var renameSlideErr error
	var editSlideErr error
	// slideCleanup switches to the slide page and deletes it.
	slideCleanup := func(ctx context.Context) error {
		return uiauto.Combine("switch to the slide page and delete it",
			uiauto.IfSuccessThen(func(ctx context.Context) error { return renameSlideErr }, switchToTab(string(googleSlides))),
			googleapps.DeleteSlide(tconn),
		)(ctx)
	}

	var renameDocErr error
	// docCleanup switches to the document page and deletes it.
	docCleanup := func(ctx context.Context) error {
		return uiauto.Combine("switch to the document page and delete it",
			uiauto.IfSuccessThen(func(ctx context.Context) error { return renameDocErr }, switchToTab(string(googleDocs))),
			googleapps.DeleteDoc(tconn),
		)(ctx)
	}
	// Shorten the context to cleanup document.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := googleapps.NewGoogleSlides(cs, tconn, extendedDisplay)(ctx); err != nil {
		return CheckSignedOutError(ctx, tconn, err)
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
	// Make sure that the Google Slides window is on the internal display.
	if extendedDisplay {
		// Switch window to internal display.
		if err := cuj.SwitchWindowToDisplay(ctx, tconn, kb, false)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch Google Slides to the internal display")
		}
	}
	renameSlideErr = googleapps.RenameSlide(tconn, kb, testTitle)(ctx)
	if renameSlideErr != nil {
		return CheckSignedOutError(ctx, tconn, renameSlideErr)
	}
	editSlideErr = googleapps.EditSlideTitle(tconn, kb, slideTitle, slideSubTitle)(ctx)
	if editSlideErr != nil {
		return CheckSignedOutError(ctx, tconn, editSlideErr)
	}
	for i := 0; i < slideCount; i++ {
		title := fmt.Sprintf(subSlideTitle+" page %d", i+1)
		content := fmt.Sprintf(subSlideContent+" page %d", i+1)
		pageNumber := strconv.Itoa(i + 2)
		editSlideErr = googleapps.NewSlide(tconn, kb, title, content, pageNumber)(ctx)
		if editSlideErr != nil {
			return CheckSignedOutError(ctx, tconn, editSlideErr)
		}
	}

	if err := googleapps.NewGoogleDocs(cs, tconn, extendedDisplay)(ctx); err != nil {
		return CheckSignedOutError(ctx, tconn, err)
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
	// Make sure that the Google Docs window is on the internal display.
	if extendedDisplay {
		// Switch window to internal display.
		if err := cuj.SwitchWindowToDisplay(ctx, tconn, kb, false)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch Google Docs to the internal display")
		}
	}

	renameDocErr = googleapps.RenameDoc(tconn, kb, testTitle)(ctx)
	if renameDocErr != nil {
		return CheckSignedOutError(ctx, tconn, renameDocErr)
	}
	testing.ContextLog(ctx, "Share screen and present ", application)
	if err := uiauto.Combine("share screen and present "+string(application),
		shareScreen,
		switchTabIfNeeded,
		presentApplication,
		stopPresenting,
	)(ctx); err != nil {
		return CheckSignedOutError(ctx, tconn, err)
	}
	return nil
}
