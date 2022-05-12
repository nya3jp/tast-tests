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
		Func:         OpenPersonalizationHubFromLauncher,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test opening personalization hub app by searching in launcher",
		Contacts: []string{
			"pzliu@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "personalizationDefault",
	})
}

func OpenPersonalizationHubFromLauncher(ctx context.Context, s *testing.State) {
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

	if err := personalization.SearchForAppInLauncher("change wallpaper", "Change wallpaper", kb, ui)(ctx); err != nil {
		s.Fatal("Failed to open wallpaper app from launcher: ", err)
	}

	if err := ui.WaitUntilExists(nodewith.Role(role.Button).NameContaining("Wallpaper").HasClass("breadcrumb"))(ctx); err != nil {
		s.Fatal("Failed to verify wallpaper app is opened: ", err)
	}

	if err := personalization.SearchForAppInLauncher("personalization hub", "Personalization", kb, ui)(ctx); err != nil {
		s.Fatal("Failed to open Personalization Hub from launcher: ", err)
	}

	if err := ui.Exists(nodewith.Role(role.Window).NameContaining("Personalization").First())(ctx); err != nil {
		s.Fatal("Failed to verify personalization hub is opened: ", err)
	}
}
