// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		Func:         TransitionState,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Create a screencast and verifies it goes through uploading to uploaded state",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "ondevice_speech"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Timeout:      10 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func TransitionState(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	tconn := s.FixtValue().(*projector.FixtData).TestConn

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	cleanup, err := projector.SetUpProjectorApp(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up Projector app: ", err)
	}
	defer cleanup(cleanupCtx)

	// We need to clean up any screencasts after the test to
	// prevent taking up Drive quota over time.
	defer projector.DeleteScreencastItems(cleanupCtx, tconn)
	if err := projector.LaunchCreationFlow(ctx, tconn, false /*launchAnnotator*/); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(2 * time.Minute)
	// New created screenast will always be the first.
	screencastItem := nodewith.ClassName("screencast-media").Role(role.GenericContainer).First()
	progressRing := nodewith.ClassName("progress-ring").Ancestor(screencastItem)
	bottomBarMenu := nodewith.Name("More options").Ancestor(screencastItem)
	s.Log("Wait for uploading finishes")
	if err := uiauto.Combine("Wait for uploading finishes",
		// Wait for uploading status:
		ui.WaitUntilExists(progressRing),
		// Wait for transcoding status:
		ui.WaitUntilGone(progressRing),
		ui.WaitUntilExists(bottomBarMenu),
	)(ctx); err != nil {
		s.Fatal("Failed to wait for uploading finishes: ", err)
	}
}
