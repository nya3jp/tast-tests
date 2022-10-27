// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const appVersiontimeoutUI = 30 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppVersion,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that app version is available from app info page",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func AppVersion(ctx context.Context, s *testing.State) {

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to PlayStore and Close.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed connecing to ARC: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml",
				filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := openAppInfoPage(ctx, tconn); err != nil {
		s.Fatal("Failed to open app info page: ", err)
	}

	if err := verifyAppVersion(ctx, d, tconn); err != nil {
		s.Fatal("Failed verifying app Version: ", err)
	}
}

// openAppInfoPage opens app info page of PlayStore.
func openAppInfoPage(ctx context.Context, tconn *chrome.TestConn) error {

	// Open App info page by right click on Play Store App.
	settings := nodewith.Name("Settings").Role(role.Window).First()
	playstoreSubpage := "Play Store subpage back button"

	ui := uiauto.New(tconn)
	playstoreSubpageButton := nodewith.Name(playstoreSubpage).Role(role.Button).Ancestor(settings)
	appInfoMenu := nodewith.Name("App info").Role(role.MenuItem)

	openPlayStoreAppInfoPage := func() uiauto.Action {
		return uiauto.Combine("check app context menu and settings",
			ui.LeftClick(appInfoMenu),
			ui.WaitUntilExists(playstoreSubpageButton))
	}

	moreSettingsButton := nodewith.Name("More settings and permissions").Role(role.Link)
	if err := uiauto.Combine("check context menu of play store app on the shelf",
		ash.RightClickApp(tconn, apps.PlayStore.Name),
		openPlayStoreAppInfoPage(),
		ui.LeftClick(moreSettingsButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to open app info for Play Store app")
	}
	return nil
}

// verifyAppVersion check that version is present in app info page under Advanced.
func verifyAppVersion(ctx context.Context, d *androidui.Device,
	tconn *chrome.TestConn) error {

	// Click on Advanced to expand it.
	advancedSettings := d.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)Advanced"), androidui.Enabled(true))
	if err := advancedSettings.WaitForExists(ctx, appVersiontimeoutUI); err != nil {
		return errors.Wrap(err, "failed to find Advanced")
	}
	if err := advancedSettings.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Advanced")
	}

	// Scroll until the version is displayed.
	scrollLayout := d.Object(androidui.ClassName("android.support.v7.widget.RecyclerView"), androidui.Scrollable(true))
	if t, ok := arc.Type(); ok && t == arc.VM {
		scrollLayout = d.Object(androidui.ClassName("androidx.recyclerview.widget.RecyclerView"), androidui.Scrollable(true))
	}
	system := d.Object(androidui.ClassName("android.widget.TextView"), androidui.TextContains("(?i)Modify system settings"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, appVersiontimeoutUI); err == nil {
		scrollLayout.ScrollTo(ctx, system)
	}

	// Verify version is not empty.
	versionText, err := d.Object(androidui.ID("android:id/summary"), androidui.TextStartsWith("version")).GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to ger version")
	}
	if len(versionText) == 0 {
		return errors.Wrap(err, "version is empty")
	}
	testing.ContextLogf(ctx, "App Version = %s", versionText)
	return nil
}
