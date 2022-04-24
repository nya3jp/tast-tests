// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
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
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Vars: []string{
			"ui.gaiaPoolDefault",
		},
	})
}

func CreationFlow(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableFeatures("Projector"),
		chrome.EnableFeatures("ProjectorAppDebug"),
		chrome.EnableFeatures("ProjectorAnnotator"),
		chrome.EnableFeatures("ProjectorTutorialVideoView"),
		// We must disable extended features because not every
		// test device currently supports SODA.
		chrome.ExtraArgs("--projector-extended-features-disabled"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	closeOnboardingButton := nodewith.Name("Skip tour").Role(role.Button)
	maximizeButton := nodewith.Name("Maximize").Role(role.Button)
	// TODO(b/229026639): Rename to "New screencast" button.
	newRecordingButton := nodewith.Name("New recording").First()
	projectorSession := nodewith.Name("Click anywhere to record full screen").Role(role.StaticText)
	annotatorButton := nodewith.Name("Screencast tools").Role(role.Button)
	annotatorFrame := nodewith.ClassName("marker-iframe").Role(role.Iframe)
	blueMarkerButton := nodewith.Name("Blue").Role(role.Button)
	stopRecordingButton := nodewith.Name("Stop screen recording").Role(role.Button)
	tutorialsText := nodewith.Name("Get started by watching these tips").Role(role.StaticText)
	closeTutorialsButton := nodewith.Name("Collapse tutorial videos").Role(role.Button)

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

	s.Log("Lauching the new screencast creation flow")
	if err := uiauto.Combine("launch the new screencast creation flow",
		ui.LeftClick(maximizeButton),
		ui.WaitUntilExists(newRecordingButton),
		ui.LeftClickUntil(newRecordingButton, ui.Gone(newRecordingButton)),
		ui.WaitUntilExists(projectorSession),
		ui.LeftClickUntil(projectorSession, ui.Gone(projectorSession)),
		ui.WaitUntilExists(annotatorButton),
		// Open the annotator.
		ui.LeftClickUntil(annotatorButton, ui.Exists(annotatorFrame)),
		// Open the color picker.
		ui.RightClickUntil(annotatorButton, ui.Exists(blueMarkerButton)),
		// Change marker color to blue.
		ui.LeftClickUntil(blueMarkerButton, ui.Gone(blueMarkerButton)),
		// Close the annotator.
		ui.LeftClickUntil(annotatorButton, ui.Gone(annotatorFrame)),
		// This test saves the screencast to the downloads
		// directory, and since the test device is
		// power-washed in between test iterations, these
		// screencasts should not take up disk space over
		// time.
		// TODO(b/229631680): Figure out how to mock DriveFS
		// and modify this test to go through the prod
		// creation flow, instead of saving to the downloads
		// directory.
		ui.WaitUntilExists(stopRecordingButton),
		ui.LeftClickUntil(stopRecordingButton, ui.Gone(stopRecordingButton)),
		// Verify the tutorial videos exist and dismiss them.
		ui.WaitUntilExists(tutorialsText),
		ui.WaitUntilExists(closeTutorialsButton),
		ui.LeftClickUntil(closeTutorialsButton, ui.Gone(tutorialsText)),
	)(ctx); err != nil {
		s.Fatal("Failed to go through the new screencast creation flow: ", err)
	}
}
