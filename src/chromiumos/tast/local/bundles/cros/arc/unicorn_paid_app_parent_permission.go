// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
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
		Func:         UnicornPaidAppParentPermission,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks if paid app installation triggers Parent Permission For Unicorn Account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"arc.parentUser"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Fixture: "familyLinkUnicornArcPolicyLogin",
	})
}

func UnicornPaidAppParentPermission(ctx context.Context, s *testing.State) {
	const (
		askinMessageButtonText = "Ask in a message"
		askinPersonButtonText  = "Ask in person"
		playStoreSearchText    = "Search for apps & games"
		gamesAppName           = "the wonder weeks"
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
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	searchText := d.Object(ui.ClassName("android.widget.TextView"), ui.Text(playStoreSearchText))
	if err := searchText.WaitForExists(ctx, 90*time.Second); err != nil {
		s.Fatal("searchText doesn't exist: ", err)
	}
	if err := searchText.Click(ctx); err != nil {
		s.Fatal("Failed to click on searchText: ", err)
	}

	searchTextEdit := d.Object(ui.ClassName("android.widget.EditText"), ui.Text(playStoreSearchText))
	if err := searchTextEdit.SetText(ctx, gamesAppName); err != nil {
		s.Fatal("Failed to set text to search: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to click on KEYCODE_ENTER button: ", err)
	}

	searchResult := d.Object(ui.ClassName("android.view.View"), ui.DescriptionContains("$"), ui.Index(1))
	if err := searchResult.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Log("Search Result doesn't exist: ", err)
	} else if err := searchResult.Click(ctx); err != nil {
		s.Fatal("Failed to click on Search Result: ", err)
	}

	installButton := d.Object(ui.ClassName("android.widget.Button"), ui.TextContains("$"), ui.Enabled(true))
	if err := installButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Install Button doesn't exisit: ", err)
	}
	if err := installButton.Click(ctx); err != nil {
		s.Fatal("Failed to click  installButton: ", err)
	}

	buyButton := d.Object(ui.ClassName("android.widget.Button"), ui.Text("Buy"), ui.Enabled(true))
	if err := buyButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("Buy Button doesn't exisit: ", err)
	}
	if err := buyButton.Click(ctx); err != nil {
		s.Fatal("Failed to click Buy Button: ", err)
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
