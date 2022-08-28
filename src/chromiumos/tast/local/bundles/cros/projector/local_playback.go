// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalPlayback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests local playback feature from DriveFS while screencast is transcoding",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Timeout:      10 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func LocalPlayback(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	if err := projector.DismissOnboardingDialog(ctx, tconn); err != nil {
		s.Fatal("Failed to close the onboarding dialog: ", err)
	}

	// We need to clean up any screencasts after the test to
	// prevent taking up Drive quota over time.
	defer projector.DeleteScreencastItems(ctxForCleanUp, tconn)

	if err := projector.LaunchCreationFlow(ctx, tconn, false /*launchAnnotator*/); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}

	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	viewerTitle := nodewith.Name("Screencast title").Role(role.TextField)
	zeroTimeElapsed := nodewith.Name("00:00").Role(role.StaticText).Ancestor(nodewith.Name("Time elapsed"))
	playButton := nodewith.Name("Play").Role(role.Button)
	homeButton := nodewith.Name("Back to home").Role(role.Button)

	s.Log("Viewing local video playback while screencast is still transcoding")
	if err := uiauto.Combine("view local video playback while screencast is still transcoding",
		ui.WithInterval(time.Second).LeftClickUntil(screencastItem, ui.Exists(viewerTitle)),
		ui.WaitUntilExists(zeroTimeElapsed),
		ui.WaitUntilExists(playButton),
		// Verify that the play head has proceeded by playing until the elapsed time is no longer 00:00.
		ui.WithInterval(time.Second).LeftClickUntil(playButton, ui.Gone(zeroTimeElapsed)),
		// Return to the gallery view to delete the new screencast.
		ui.WithInterval(time.Second).LeftClickUntil(homeButton, ui.Gone(viewerTitle)),
	)(ctx); err != nil {
		s.Fatal("Failed to view local transcoding screencast video: ", err)
	}
}
