// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenWithMenuChrome,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test ARC's open with menu show up in Chrome's right click menu",
		Contacts:     []string{"elkurin@chromium.org", "lacros-tok@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}},
	})
}

func OpenWithMenuChrome(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	arcDevice := s.FixtValue().(*arc.PreData).ARC

	const (
		appName        = "Intent Picker Test App"
		intentActionID = "org.chromium.arc.testapp.chromeintentpicker:id/intent_action"
		expectedAction = "android.intent.action.VIEW"
	)

	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Setup ARC and Installs APK.
	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeIntentPickerTest.apk")); err != nil {
		s.Fatal("Failed installing the APK: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open search result page
	conn, err := br.NewConn(ctx, "https://google.com/search?q=Google")
	if err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}
	defer conn.Close()

	// Wait for Google Logo to appear
	googleLogo := nodewith.Name("Google").Role(role.Link).First()
	if err := ui.WaitUntilExists(googleLogo)(ctx); err != nil {
		s.Fatal("Failed to wait for Google Logo to load: ", err)
	}

	// Get the Google Logo next to search bar.
	googleLogoOption := nodewith.Name("Open with Intent Picker Test App").Role(role.MenuItem)
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(googleLogo),
		ui.WaitUntilExists(googleLogoOption))(ctx); err != nil {
		s.Log("Failed to show context menu of Google Logo: ", err)
		// After timeout, dump all the menuItems if possible, this should provide a clear
		// idea whether items are missing in the menu or the menu not being there at all.
		menu := nodewith.ClassName("MenuItemView")
		menuItems, err := ui.NodesInfo(ctx, menu)
		if err != nil {
			s.Fatal("Could not find context menu items: ", err)
		}
		var items []string
		for _, item := range menuItems {
			items = append(items, item.Name)
		}
		s.Fatalf("Found %d menu items, including: %s", len(items), strings.Join(items, " / "))
	}
}
