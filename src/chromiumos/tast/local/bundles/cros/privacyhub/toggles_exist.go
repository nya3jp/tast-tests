// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhub contains tests for privacy hub
package privacyhub

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TogglesExist,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that PrivacyHub settings page contains the expected toggles",
		Contacts:     []string{"janlanik@google.com", "privacy-hub@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
	})
}

func TogglesExist(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("CrosPrivacyHub"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Check that the Privacy Hub section contains a button for camera
	cameraLabel := nodewith.Name("Camera").Role(role.ToggleButton)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrivacy/privacyHub", uiauto.New(tconn).WaitUntilExists(cameraLabel))
	defer settings.Close(cleanupCtx)
	if err != nil {
		s.Fatal("Failed to find Camera in Privacy Hub page: ", err)
	}
}
