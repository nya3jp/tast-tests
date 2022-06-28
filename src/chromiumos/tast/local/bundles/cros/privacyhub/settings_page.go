// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhub contains tests for privacy hub.
package privacyhub

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
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

func checkMenuExists(ctx *context.Context, tconn *chrome.TestConn, menu *nodewith.Finder) error {
	if err := uiauto.New(tconn).WaitUntilExists(menu)(*ctx); err != nil {
		return errors.Wrap(err, "failed to find Privacy Hub in OS setting page")
	}
	return nil
}

func checkMenuDoesNotExist(ctx *context.Context, tconn *chrome.TestConn, menu *nodewith.Finder) error {
	if err := uiauto.New(tconn).WaitUntilExists(menu)(*ctx); err == nil {
		return errors.New("found Privacy Hub in OS setting page even though the flag is not set")
	}
	return nil
}

func checkToggles(cleanupCtx, ctx *context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	cameraLabel := nodewith.Name("Camera").Role(role.ToggleButton)
	settings, err := ossettings.LaunchAtPageURL(*ctx, tconn, cr, "osPrivacy/privacyHub", uiauto.New(tconn).WaitUntilExists(cameraLabel))
	if err != nil {
		return errors.New("failed to find Camera in Privacy Hub page")
	}
	defer settings.Close(*cleanupCtx)
	return nil
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

	privacyMenu := nodewith.Name("Privacy Hub")
	if featureOn {
		// Check that the Privacy Hub section exists.
		if err = checkMenuExists(&ctx, tconn, privacyMenu); err != nil {
			s.Fatal("Failed to verify PrivacyHub menu existence: ", err)
		}
		// Check that the Privacy Hub section contains a button for camera.
		if err = checkToggles(&cleanupCtx, &ctx, tconn, cr); err != nil {
			s.Fatal("Failed to verify all PrivacyHub menu toggles: ", err)
		}
	} else {
		// Check that the Privacy Hub section does not exist if feature flag not explicitly set.
		// This will be removed when PrivacyHub is in production.
		privacyMenu := nodewith.Name("Privacy Hub")
		if err = checkMenuDoesNotExist(&ctx, tconn, privacyMenu); err != nil {
			s.Fatal("Failed to verify PrivacyHub menu non-existence: ", err)
		}

	}
}
