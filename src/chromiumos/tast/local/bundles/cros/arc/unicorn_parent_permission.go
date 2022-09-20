// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornParentPermission,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if App Install Triggers Parent Permission For Unicorn Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		VarDeps:      []string{"arc.parentUser"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcPolicyLogin",
	})
}

func UnicornParentPermission(ctx context.Context, s *testing.State) {
	const (
		askinMessageButtonText = "Ask in a message"
		askinPersonButtonText  = "Ask in person"
		installButtonText      = "install"
		playStoreSearchText    = "Search for apps & games"
		appName                = "Instagram"
	)
	parentUser := s.RequiredVar("arc.parentUser")
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn := s.FixtValue().(familylink.HasTestConn).TestConn()

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
	defer apps.Close(ctx, tconn, apps.PlayStore.ID)

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

	// Try on Install Some Games App.
	searchText := d.Object(ui.ClassName("android.widget.TextView"), ui.Text(playStoreSearchText))
	if err := searchText.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("searchText doesn't exist: ", err)
	}
	if err := searchText.Click(ctx); err != nil {
		s.Fatal("Failed to click on searchText: ", err)
	}

	searchTextEdit := d.Object(ui.ClassName("android.widget.EditText"), ui.Text(playStoreSearchText))
	if err := searchTextEdit.SetText(ctx, appName); err != nil {
		s.Fatal("Failed to set text to search: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextMatches("(?i)"+installButtonText), ui.Enabled(true))
	if err := installButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Install Button Exisits: ", err)
	}
	if err := installButton.Click(ctx); err != nil {
		s.Fatal("Failed to click  installButton: ", err)
	}

	// Verify Parent Permission Dialog is displayed.
	askinPersonButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text(askinPersonButtonText), ui.Enabled(true))
	if err := askinPersonButton.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("Ask in person button doesn't Exists: ", err)
	}

	if err := d.Object(ui.TextMatches(askinMessageButtonText)).Exists(ctx); err != nil {
		s.Fatal("Ask in a message button doesn't exist: ", err)
	}

	if err = askinPersonButton.Click(ctx); err != nil {
		s.Fatal("Failed to click  Ask in person: ", err)
	}

	parentPwd := d.Object(ui.ClassName("android.widget.EditText"), ui.Text(parentUser))
	if err := parentPwd.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("parentPwd doesn't Exists: ", err)
	}

}
