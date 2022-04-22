// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/projector"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SharedScreencast,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Opens a shared screencast in viewer mode",
		Contacts:     []string{"tobyhuang@chromium.org", "cros-projector@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "projectorLoginExtendedFeaturesDisabled",
		VarDeps: []string{
			"projector.sharedScreencast",
		},
	})
}

func SharedScreencast(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*projector.FixtData).TestConn
	cr := s.FixtValue().(*projector.FixtData).Chrome

	sharedScreencast := s.RequiredVar("projector.sharedScreencast")

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(time.Minute)

	closeOnboardingButton := nodewith.Name("Skip tour").Role(role.Button)
	screencastTitle := nodewith.Name("Screencast for Tast (Do not modify)").Role(role.StaticText)

	// Set up browser.
	// TODO(b/229633861): Also test URL handling in Lacros browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer closeBrowser(ctx)

	// Open a new window.
	conn, err := br.NewConn(ctx, sharedScreencast, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to navigate to Projector landing page: ", err)
	}
	defer conn.Close()

	if err := br.ReloadActiveTab(ctx); err != nil {
		s.Fatal("Failed to launch Projector app: ", err)
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

	// Verify the shared screencast rendered correctly.
	if err := ui.WaitUntilExists(screencastTitle)(ctx); err != nil {
		s.Fatal("Failed to render shared screencast: ", err)
	}
}
