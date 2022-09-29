// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CreationFlow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Launches the Projector app and goes through the new screencast creation flow with annotator",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Timeout:      10 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func CreationFlow(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 4*time.Minute)
	defer cancel()

	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

	defer faillog.DumpUITreeOnError(ctxForCleanUp, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)

	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	tutorialsText := nodewith.Name("Getting started").Role(role.StaticText)
	closeTutorialsButton := nodewith.Name("Close tutorials").Role(role.Button)

	cleanup, err := projector.SetUpProjectorApp(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up Projector app: ", err)
	}
	defer cleanup(ctxForCleanUp)

	// We need to clean up any screencasts after the test to
	// prevent taking up Drive quota over time.
	defer projector.DeleteScreencastItems(ctxForCleanUp, tconn)

	s.Log("Setting up the new screencast creation flow")
	if err := uiauto.Combine("Setting up the new screencast creation flow",
		// Make sure there are no existing screencasts before
		// starting the test.
		ui.Gone(screencastItem),
		// Dismiss the tutorial videos in case they hide the screencast item on small screens.
		ui.WaitUntilExists(closeTutorialsButton),
		ui.LeftClickUntil(closeTutorialsButton, ui.Gone(tutorialsText)),
	)(ctx); err != nil {
		s.Fatal("Failed to set up the new screencast creation flow: ", err)
	}

	if err := projector.LaunchCreationFlow(ctx, tconn, true /*launchAnnotator*/); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}
}
