// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
)

// RefreshApp returns an action that refreshes the Screencast app by right-clicking.
// TODO(b/231097154): Refreshing in a loop should not be necessary.
// Replace with WaitUntilExists() once this bug has been fixed.
func RefreshApp(ctx context.Context, tconn *chrome.TestConn) uiauto.Action {
	ui := uiauto.New(tconn)
	appWindow := nodewith.ClassName("WebAppFrameToolbarView").Role(role.Pane)
	reload := nodewith.Name("Reload Ctrl+R").Role(role.MenuItem)

	return func(ctx context.Context) error {
		if err := uiauto.Combine("refresh app through right-click context menu",
			ui.RightClickUntil(appWindow, ui.Exists(reload)),
			ui.LeftClick(reload),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to refresh app")
		}
		return nil
	}
}

// SetUpProjectorApp launches the Projector aka Screencast app from
// the launcher and dimisses the onboarding dialog. It also checks the
// microphone and sets up a fake one if necessary. The caller should
// schedule a deferred call to the returned cleanup function to unload
// aloop if err is nil.
func SetUpProjectorApp(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context), error) {
	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open Projector app")
	}

	if err := DismissOnboardingDialog(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to close the onboarding dialog")
	}

	if err := audio.WaitForDevice(ctx, audio.InputStream); err == nil {
		return func(ctx context.Context) {}, nil
	}

	testing.ContextLog(ctx, "Microphone is unavailable, verifying new screencast button is disabled")
	if err := VerifyNewScreencastButtonDisabled(ctx, tconn, "Turn on microphone"); err != nil {
		return nil, errors.Wrap(err, "microphone is unavailable, but new screencast button is enabled")
	}

	// Set up CRAS Aloop for audio test. Set up a fake microphone
	// so the test may proceed.
	cleanup, err := voice.EnableAloop(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to enable aloop")
	}

	return cleanup, nil
}

// DismissOnboardingDialog closes the onboarding dialog if it exists.
func DismissOnboardingDialog(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	closeOnboardingButton := nodewith.Name("Close").Role(role.Button).FinalAncestor(nodewith.Role(role.Dialog))

	// Since each user only sees the onboarding flow a maximum of three
	// times, the onboarding dialog may not appear.
	if err := ui.WaitUntilExists(closeOnboardingButton)(ctx); err != nil {
		// Onboarding dialog not found.
		return nil
	}

	testing.ContextLog(ctx, "Dismissing the onboarding dialog")
	if err := ui.LeftClickUntil(closeOnboardingButton, ui.Gone(closeOnboardingButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to close the onboarding dialog")
	}
	return nil
}

// VerifyNewScreencastButtonDisabled verifies that the new screencast
// exists but it is disabled with the specified error tooltip.
func VerifyNewScreencastButtonDisabled(ctx context.Context, tconn *chrome.TestConn, tooltipText string) error {
	ui := uiauto.New(tconn)
	newScreencastButton := nodewith.Name("New screencast").Role(role.Button)
	errorTooltip := nodewith.Name(tooltipText).Role(role.GenericContainer)
	refreshApp := RefreshApp(ctx, tconn)
	if err := uiauto.Combine("verify the new screencast button is disabled",
		ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(newScreencastButton)),
		// The new screencast button exists but it is not enabled.
		ui.Gone(newScreencastButton.Focusable()),
		ui.Exists(errorTooltip),
	)(ctx); err != nil {
		return errors.Wrapf(err, "new screencast button is not disabled with expected error: %s", tooltipText)
	}
	return nil
}

// LaunchCreationFlow creates a new screencast. Don't forget to call
// DeleteScreencastItems() at the end of your test to clean up, or
// else the screencasts will take up Drive quota over time.
func LaunchCreationFlow(ctx context.Context, tconn *chrome.TestConn, launchAnnotator bool) error {
	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	newScreencastButton := nodewith.Name("New screencast").Role(role.Button).Focusable()
	clickOrTapRegex := regexp.MustCompile("(Click|Tap) anywhere to record full screen")
	clickOrTapAnywhereToRecord := nodewith.NameRegex(clickOrTapRegex).Role(role.StaticText)
	stopRecordingButton := nodewith.Name("Stop screen recording").Role(role.Button)

	testing.ContextLog(ctx, "Starting the new screencast creation flow")
	if err := uiauto.Combine("starting the new screencast creation flow",
		ui.WaitUntilExists(newScreencastButton),
		// Expect the Screencast app to minimize once the
		// recording session starts, so the button should
		// disappear.
		ui.LeftClickUntil(newScreencastButton, ui.Gone(newScreencastButton)),
		ui.WaitUntilExists(clickOrTapAnywhereToRecord),
		ui.LeftClickUntil(clickOrTapAnywhereToRecord, ui.Gone(clickOrTapAnywhereToRecord)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to start new screencast creation flow")
	}

	if launchAnnotator {
		if err := LaunchAnnotator(ctx, tconn); err != nil {
			return err
		}
	}

	// Ensure the recording is at least a couple seconds long.
	testing.Sleep(ctx, 6*time.Second)

	testing.ContextLog(ctx, "Stopping recording")
	if err := uiauto.Combine("stopping recording",
		ui.WaitUntilExists(stopRecordingButton),
		// Expect the Screencast app to maximize after
		// recording stops.
		ui.LeftClickUntil(stopRecordingButton, ui.Gone(stopRecordingButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	return nil
}

// LaunchAnnotator opens the annotator, changes the marker color, and
// draws a dot.
func LaunchAnnotator(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	annotatorTrayButton := nodewith.NameStartingWith("Toggle marker.").Role(role.Button)
	inkCanvas := nodewith.ClassName("ink-engine").Role(role.Canvas)
	blueMarkerButton := nodewith.Name("Blue").Role(role.Button)

	testing.ContextLog(ctx, "Launching the annotator")
	if err := uiauto.Combine("launch the annotator",
		ui.WaitUntilExists(annotatorTrayButton),
		// Enable the annotator.
		ui.WithInterval(time.Second).LeftClickUntil(annotatorTrayButton, ui.Exists(inkCanvas)),
		// Open the color picker.
		ui.RightClickUntil(annotatorTrayButton, ui.Exists(blueMarkerButton)),
		// Change marker color to blue.
		ui.LeftClickUntil(blueMarkerButton, ui.Gone(blueMarkerButton)),
		// Draw a blue dot.
		// TODO(b/229634049): Verify the drawing rendered.
		ui.LeftClick(inkCanvas),
		// Clear the canvas.
		// TODO(b/229634049): Verify the canvas cleared.
		ui.RightClick(inkCanvas),
		// Disable the annotator.
		ui.WithInterval(time.Second).LeftClickUntil(annotatorTrayButton, ui.Gone(inkCanvas)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch annotator")
	}
	return nil
}

// DeleteScreencastItems deletes all "old" screencast items in the
// gallery view. Since screencasts are sorted in reverse chronological
// order on the gallery, "old" is defined as the nth screencast item
// or older. It is important to only delete old screencast items to
// avoid interfering with concurrently running tests who are still
// using the more recent screencasts. This cleanup is important for
// freeing up Drive quota for the test accounts.
func DeleteScreencastItems(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Deleting old screencasts")

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	n := 7
	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).Nth(n)
	screencastItemMoreOptionsButton := nodewith.Name("More options").Role(role.PopUpButton).Ancestor(screencastItem)
	deleteMenuItem := nodewith.Name("Delete").Role(role.MenuItem)
	deleteButton := nodewith.Name("Delete").Role(role.Button)

	if err := ui.WaitUntilExists(screencastItem)(ctx); err != nil {
		testing.ContextLogf(ctx, "There are %d or fewer screencast items on the gallery, no deletion required", n)
		return nil
	}

	deleteScreencastItem := func(ctx context.Context) error {
		if err := uiauto.Combine(fmt.Sprintf("deleting the %dth screencast item", n+1),
			ui.WaitUntilExists(screencastItemMoreOptionsButton),
			ui.MakeVisible(screencastItemMoreOptionsButton),
			ui.LeftClickUntil(screencastItemMoreOptionsButton, ui.Exists(deleteMenuItem)),
			ui.LeftClickUntil(deleteMenuItem, ui.Exists(deleteButton)),
			ui.LeftClickUntil(deleteButton, ui.Gone(deleteButton)),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to delete %dth screencast item", n+1)
		}
		return nil
	}

	if err := ui.WithInterval(5*time.Second).RetryUntil(deleteScreencastItem, ui.Gone(screencastItem))(ctx); err != nil {
		return errors.Wrap(err, "failed to delete all old screencast items")
	}

	return nil
}
