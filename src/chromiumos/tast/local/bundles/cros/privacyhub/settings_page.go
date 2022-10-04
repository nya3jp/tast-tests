// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhub contains tests for privacy hub.
package privacyhub

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsPage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that PrivacyHub settings page exists and that it contains the expected elements",
		Contacts:     []string{"janlanik@google.com", "privacy-hub@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name: "feature_on",
				Val:  true,
			},
			{
				Name: "feature_off",
				Val:  false,
			},
		},
	})
}

func SettingsPage(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	featureOn := s.Param().(bool)

	var cr *chrome.Chrome
	var err error
	if featureOn {
		cr, err = chrome.New(ctx, chrome.EnableFeatures("CrosPrivacyHub"))
	} else {
		cr, err = chrome.New(ctx)
	}
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn)
	privacyMenu := nodewith.Name("Privacy Hub")
	if featureOn {
		if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(privacyMenu)(ctx); err != nil {
			s.Fatal("Failed to find Privacy Hub in OS setting page: ", err)
		}
		// Check that the Privacy Hub section contains the required buttons.
		cameraLabel := nodewith.Name("Camera").Role(role.ToggleButton)
		microphoneLabel := nodewith.Name("Microphone").Role(role.ToggleButton)
		if err := uiauto.Combine("Verify privacy menu page",
			ui.DoDefault(privacyMenu),
			ui.WaitUntilExists(cameraLabel),
			ui.WaitUntilExists(microphoneLabel),
		)(ctx); err != nil {
			s.Fatal("Failed to verify privacy menu: ", err)
		}
	} else {
		// Check that the Privacy Hub section does not exist if feature flag is not explicitly set.
		// This will be removed when PrivacyHub is in production.
		if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(privacyMenu)(ctx); err == nil {
			s.Fatal("Found Privacy Hub in OS setting page even though the flag is not set: ")
		}
	}
}
