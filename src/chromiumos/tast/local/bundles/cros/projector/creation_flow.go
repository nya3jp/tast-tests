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

	appWindow := nodewith.Name("Screencast").Role(role.Application)
	reload := nodewith.Name("Reload Ctrl+R").Role(role.MenuItem)
	maximizeButton := nodewith.Name("Maximize").Role(role.Button)
	newScreencastButton := nodewith.Name("New screencast").Role(role.Button)
	clickAnywhereToRecord := nodewith.Name("Click anywhere to record full screen").Role(role.StaticText)
	annotatorTrayButton := nodewith.NameStartingWith("Toggle marker.").Role(role.Button)
	inkCanvas := nodewith.ClassName("ink-engine").Role(role.Canvas)
	blueMarkerButton := nodewith.Name("Blue").Role(role.Button)
	stopRecordingButton := nodewith.Name("Stop screen recording").Role(role.Button)
	tutorialsText := nodewith.Name("Getting started").Role(role.StaticText)
	closeTutorialsButton := nodewith.Name("Close tutorials").Role(role.Button)
	screencastItemMoreOptionsButton := nodewith.Name("More options").Role(role.PopUpButton).Nth(1)
	deleteMenuItem := nodewith.Name("Delete").Role(role.MenuItem)
	deleteButton := nodewith.Name("Delete").Role(role.Button)

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	// Refresh the app until the new screencast and close tutorials buttons exist.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := uiauto.Combine("refresh app",
			ui.RightClickUntil(appWindow, ui.Exists(reload)),
			ui.LeftClick(reload),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to refresh app")
		}
		if err := ui.Exists(newScreencastButton)(ctx); err != nil {
			return errors.Wrap(err, "new screencast button still doesn't exist")
		}
		if err := ui.Exists(closeTutorialsButton)(ctx); err != nil {
			return errors.Wrap(err, "close tutorials button still doesn't exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Minute, Interval: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for new screencast and close tutorials buttons to appear: ", err)
	}

	s.Log("Launching the new screencast creation flow")
	if err := uiauto.Combine("launch the new screencast creation flow",
		// Maximize the app to mitigate screen size
		// differences on different devices.
		ui.LeftClick(maximizeButton),
		ui.WaitUntilExists(newScreencastButton),
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
		ui.LeftClickUntil(stopRecordingButton, ui.Gone(stopRecordingButton)),
		// Dismiss the tutorial videos in case they block the screencast item.
		ui.WaitUntilExists(closeTutorialsButton),
		ui.LeftClickUntil(closeTutorialsButton, ui.Gone(tutorialsText)),
		// We need to clean up this screencast to prevent
		// taking up Drive quota over time.
		ui.WaitUntilExists(screencastItemMoreOptionsButton),
		ui.MakeVisible(screencastItemMoreOptionsButton),
		ui.LeftClickUntil(screencastItemMoreOptionsButton, ui.Exists(deleteMenuItem)),
		ui.LeftClickUntil(deleteMenuItem, ui.Exists(deleteButton)),
		ui.LeftClickUntil(deleteButton, ui.Gone(deleteButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}
}
