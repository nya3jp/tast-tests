// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast-tests/local/personalization"
	"go.chromium.org/chromiumos/tast/testing"
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
		Fixture:      "personalizationDefault",
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
		personalization.SearchForAppInLauncher("settings", "Settings, Installed App", kb, ui),
		ui.LeftClick(nodewith.Role(role.Link).NameContaining("Personalization").HasClass("item")),
		ui.LeftClick(nodewith.Role(role.Link).NameContaining("Set your wallpaper").First()),
		ui.WaitUntilExists(nodewith.Role(role.Window).NameContaining("Wallpaper & style").First()),
	)(ctx); err != nil {
		s.Fatal("Failed to open personalization hub from settings: ", err)
	}

	if err := uiauto.Combine("open personalization hub by searching in settings",
		personalization.SearchForAppInLauncher("settings", "Settings, Installed App", kb, ui),
		ui.WaitUntilExists(nodewith.Role(role.TextField).HasClass("Textfield")),
		kb.TypeAction("personalization"),
		kb.AccelAction("Enter"),
		ui.WaitUntilExists(nodewith.Role(role.Window).NameContaining("Wallpaper & style").First()),
	)(ctx); err != nil {
		s.Fatal("Failed to open personalization hub by searching in settings: ", err)
	}
}
