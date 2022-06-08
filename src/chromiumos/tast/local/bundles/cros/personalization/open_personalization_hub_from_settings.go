// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenPersonalizationHubFromSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test opening personalization hub app from Settings app",
		Contacts: []string{
			"pzliu@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func OpenPersonalizationHubFromSettings(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Combine("open personalization hub from settings",
		personalization.SearchForAppInLauncher(personalization.SettingsSearchTerm, personalization.SettingsAppName, kb, ui),
		ui.LeftClick(nodewith.Role(role.Link).NameContaining(personalization.Personalization).HasClass("item")),
		ui.LeftClick(nodewith.Role(role.Link).NameContaining(personalization.SettingsSetWallpaper).First()),
		ui.WaitUntilExists(personalization.PersonalizationHubWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to open personalization hub from settings: ", err)
	}

	if err := uiauto.Combine("open personalization hub by searching in settings",
		personalization.SearchForAppInLauncher(personalization.SettingsSearchTerm, personalization.SettingsAppName, kb, ui),
		ui.WaitUntilExists(nodewith.Role(role.TextField).HasClass("Textfield")),
		kb.TypeAction(personalization.PersonalizationSearchTerm),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(personalization.PersonalizationHubWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to open personalization hub by searching in settings: ", err)
	}
}
