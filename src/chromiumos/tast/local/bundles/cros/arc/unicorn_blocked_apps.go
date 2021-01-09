// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornBlockedApps,
		Desc:         "Checks if blocked apps cannot be installed from Child Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcLogin",
	})
}

func UnicornBlockedApps(ctx context.Context, s *testing.State) {
	const (
		pkgName                = "com.google.android.apps.maps"
		askinMessageButtonText = "Ask in a message"
		askinPersonButtonText  = "Ask in person"
		DefaultUITimeout       = 60 * time.Second
		searchID               = "com.android.vending:id/search_bar_hint"
		installButtonText      = "install"
	)

	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	st, err := arc.GetState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get ARC state: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	if st.Provisioned {
		s.Log("ARC is already provisioned. Skipping the Play Store setup")
		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to close the provisioned Play Store: ", err)
		}
	} else {
		// Optin to Play Store.
		s.Log("Opting into Play Store")
		if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to optin to Play Store and Close: ", err)
		}
	}
	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch Play Store")
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Verify PlayStore is Open.
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		// Ensure Play Store is not empty.
		s.Log("Starting Play Store")
		act, err := arc.NewActivity(a, "com.android.vending", "com.android.vending.AssetBrowserActivity")
		if err != nil {
			s.Fatal("Failed to create new activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed starting Play Store or Play Store is empty: ", err)
		}
	}

	searchText := d.Object(ui.ClassName("android.widget.TextView"), ui.Text("Search for apps & games"))
	if err := searchText.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("searchText doesn't exist: ", err)
	} else if err := searchText.Click(ctx); err != nil {
		s.Fatal("Failed to click on searchText: ", err)
	}

	searchTextEdit := d.Object(ui.ClassName("android.widget.EditText"), ui.Text("Search for apps & games"))
	if err := searchTextEdit.SetText(ctx, "youtube.creator"); err != nil {
		s.Fatal("Failed to searchText: ", err)
	} else if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	// If the install button is enabled, fail the case.
	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
	if err := installButton.Exists(ctx); err == nil {
		s.Fatal("Install Button Exisits for Blocked App: ", err)
	}

}
