// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreationFlow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches the Projector app and goes through the new screencast creation flow",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		Timeout:      10 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func CreationFlow(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	closeOnboardingButton := nodewith.Name("Skip tour").Role(role.Button)
	appWindow := nodewith.Name("Screencast").Role(role.Application)
	reload := nodewith.Name("Reload Ctrl+R").Role(role.MenuItem)
	maximizeButton := nodewith.Name("Maximize").Role(role.Button)
	newScreencastButton := nodewith.Name("New screencast").Role(role.Button)
	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	clickAnywhereToRecord := nodewith.Name("Click anywhere to record full screen").Role(role.StaticText)
	annotatorTrayButton := nodewith.NameStartingWith("Toggle marker.").Role(role.Button)
	inkCanvas := nodewith.ClassName("ink-engine").Role(role.Canvas)
	blueMarkerButton := nodewith.Name("Blue").Role(role.Button)
	stopRecordingButton := nodewith.Name("Stop screen recording").Role(role.Button)
	tutorialsText := nodewith.Name("Getting started").Role(role.StaticText)
	closeTutorialsButton := nodewith.Name("Close tutorials").Role(role.Button)

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	// Dismiss the onboarding dialog, if it exists. Since each
	// user only sees the onboarding flow a maximum of three
	// times, the onboarding dialog may not appear.
	if err := ui.WaitUntilExists(closeOnboardingButton)(ctx); err == nil {
		s.Log("Dismissing the onboarding dialog")
		if err = ui.LeftClickUntil(closeOnboardingButton, ui.Gone(closeOnboardingButton))(ctx); err != nil {
			s.Fatal("Failed to close the onboarding dialog: ", err)
		}
	}

	// UI action for refreshing the app until the element we're
	// looking for exists.
	// TODO(b/231097154): Refreshing in a loop should not
	// be necessary. Replace with WaitUntilExists() once
	// this bug has been fixed.
	refreshApp := func(ctx context.Context) error {
		if err := uiauto.Combine("refresh app",
			ui.RightClickUntil(appWindow, ui.Exists(reload)),
			ui.LeftClick(reload),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to refresh app")
		}
		return nil
	}

	// We need to clean up any screencasts after the test to
	// prevent taking up Drive quota over time.
	defer deleteScreencastItems(ctxForCleanUp, tconn)

	s.Log("Launching the new screencast creation flow")
	if err := uiauto.Combine("launch the new screencast creation flow",
		// Maximize the app to mitigate screen size
		// differences on different devices.
		ui.LeftClick(maximizeButton),
		ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(newScreencastButton)),
		// Make sure there are no existing screencasts before
		// starting the test.
		ui.Gone(screencastItem),
		// Expect the Screencast app to minimize once the
		// recording session starts, so the button should
		// disappear.
		ui.LeftClickUntil(newScreencastButton, ui.Gone(newScreencastButton)),
		ui.WaitUntilExists(clickAnywhereToRecord),
		ui.LeftClickUntil(clickAnywhereToRecord, ui.Gone(clickAnywhereToRecord)),
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
		// This test saves the screencast to the DriveFS
		// directory.
		ui.WaitUntilExists(stopRecordingButton),
		// Expect the Screencast app to maximize after
		// recording stops.
		ui.LeftClickUntil(stopRecordingButton, ui.Gone(stopRecordingButton)),
		// Dismiss the tutorial videos in case they hide the screencast item on small screens.
		ui.WithInterval(5*time.Second).RetryUntil(refreshApp, ui.Exists(closeTutorialsButton)),
		ui.LeftClickUntil(closeTutorialsButton, ui.Gone(tutorialsText)),
	)(ctx); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}
}

// deleteScreencastItems is a helper function to delete all screencast items in the gallery view.
func deleteScreencastItems(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Deleting screencasts")

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	screencastItemMoreOptionsButton := nodewith.Name("More options").Role(role.PopUpButton).Ancestor(screencastItem)
	deleteMenuItem := nodewith.Name("Delete").Role(role.MenuItem)
	deleteButton := nodewith.Name("Delete").Role(role.Button)

	deleteScreencastItem := func(ctx context.Context) error {
		if err := uiauto.Combine("delete first screencast item",
			ui.WaitUntilExists(screencastItemMoreOptionsButton),
			ui.MakeVisible(screencastItemMoreOptionsButton),
			ui.LeftClickUntil(screencastItemMoreOptionsButton, ui.Exists(deleteMenuItem)),
			ui.LeftClickUntil(deleteMenuItem, ui.Exists(deleteButton)),
			ui.LeftClickUntil(deleteButton, ui.Gone(deleteButton)),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to delete screencast item")
		}
		return nil
	}

	if err := ui.WithInterval(5*time.Second).RetryUntil(deleteScreencastItem, ui.Gone(screencastItem))(ctx); err != nil {
		return errors.Wrap(err, "failed to delete all leftover screencast items")
	}

	return nil
}
