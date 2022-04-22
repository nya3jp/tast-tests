// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks launching the Projector app from the launcher",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Fixture:      "projectorLogin",
	})
}

func OpenApp(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*projector.FixtData).TestConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := launcher.SearchAndWaitForAppOpen(tconn, kb, apps.Projector)(ctx); err != nil {
		s.Fatal("Failed to open Projector app: ", err)
	}

	// Dismiss the onboarding dialog, if it exists. Each user only sees the onboarding flow a maximum of three times.
	// TODO(b/229631476): Update the close button on the onboarding dialog to "Skip tour".
	closeButton := nodewith.Name("Learn more").Role(role.Button)
	if err := ui.WaitUntilExists(closeButton)(ctx); err == nil {
		s.Log("Dismissing the onboarding dialog")
		if err = ui.LeftClickUntil(closeButton, ui.Gone(closeButton))(ctx); err != nil {
			s.Fatal("Failed to click the close button on the onboarding dialog: ", err)
		}
	}

	// TODO(b/229632058): Enable the new recording button and launch the creation flow. The current blocker is no microphone input on VM for testing.
}
