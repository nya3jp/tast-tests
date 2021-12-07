// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ParentalControlsLink,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify 'Parental controls' opens https://families.google.com/families",
		Contacts: []string{
			"victor.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		Fixture:      "familyLinkGellerLogin",
	})
}

const familiesURL = "https://families.google.com/families"

// ParentalControlsLink verifies 'Parental controls' opens https://families.google.com/families.
func ParentalControlsLink(ctx context.Context, s *testing.State) {
	var (
		cr    = s.FixtValue().(*familylink.FixtData).Chrome
		tconn = s.FixtValue().(*familylink.FixtData).TestConn
		ui    = uiauto.New(tconn)
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Accounts").Role(role.Link))
	if err != nil {
		s.Fatal("Failed to open Accounts page: ", err)
	}
	defer settings.Close(cleanupCtx)

	parentalControls := nodewith.NameContaining("Parental controls Open").FinalAncestor(ossettings.WindowFinder)
	if err := ui.LeftClick(parentalControls)(ctx); err != nil {
		s.Fatal("Failed to open Parental controls: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		if err := browser.CloseAllTabs(ctx, tconn); err != nil {
			s.Log("Failed to close tabs: ", err)
		}
	}(cleanupCtx)

	// Wait redirect to families page.
	if err := ui.WaitUntilExists(nodewith.NameContaining("Families").HasClass("BrowserFrame"))(ctx); err != nil {
		s.Fatal("Failed to wait families page exists: ", err)
	}

	tabs, err := browser.CurrentTabs(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find current tabs: ", err)
	}

	if len(tabs) != 1 {
		s.Fatalf("Failed to verify the expected page opened: unexpected tab number: want 1, got %d", len(tabs))
	} else if tabs[0].URL != familiesURL {
		s.Fatalf("Failed to verify the expected page opened: want %q, got %q", familiesURL, tabs[0].URL)
	}
}
