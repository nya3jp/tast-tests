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
		Func: ParentalControlsLink,
		// TODO(b/250500759): Support lacros for this test once this issue is fixed.
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify 'Parental controls' setting opens https://families.google.com/families when Play Store is disabled",
		Contacts: []string{
			"victor.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"cros-families-eng+test@google.com ",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
		Fixture:      "familyLinkGellerLogin", // Expecting the ARC to be disabled in this test.
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
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump_before_close_settings")
		if err := settings.Close(ctx); err != nil {
			s.Log("Failed to close settings: ", err)
		}
	}(cleanupCtx)

	if err := uiauto.Combine("open parental controls",
		ui.LeftClick(nodewith.NameContaining("Parental controls Open").FinalAncestor(ossettings.WindowFinder)),
		ui.WaitUntilExists(nodewith.NameContaining("Families").HasClass("BrowserFrame")),
	)(ctx); err != nil {
		s.Fatal(`Failed to verify the functionality of "Parental controls" settings: `, err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump_before_close_browser")
		if err := browser.CloseAllTabs(ctx, tconn); err != nil {
			s.Log("Failed to close tabs: ", err)
		}
	}(cleanupCtx)

	tabs, err := browser.CurrentTabs(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find current tabs: ", err)
	}

	if len(tabs) != 1 {
		s.Fatalf("Failed to verify the expected page opened: unexpected tab number: want 1, got %d", len(tabs))
	}

	if tabs[0].URL != familiesURL {
		s.Fatalf("Failed to verify the expected page opened: want %q, got %q", familiesURL, tabs[0].URL)
	}
}
