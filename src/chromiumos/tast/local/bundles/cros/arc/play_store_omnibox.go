// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayStoreOmnibox,
		Desc:     "Installs a TWA and WebAPK app via Omnibox in Play Store",
		Contacts: []string{"benreich@chromium.org", "jshikaram@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arc.PlayStoreOmnibox.username", "arc.PlayStoreOmnibox.password"},
	})
}

// Time to wait for UI elements to appear in Play Store and Chrome
const uiTimeout = 30 * time.Second

func PlayStoreOmnibox(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.PlayStoreOmnibox.username")
	password := s.RequiredVar("arc.PlayStoreOmnibox.password")

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Setup Chrome Test API Connection
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

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
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Navigate to URL
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	for _, tc := range []struct {
		title     string
		publisher string
		url       string
	}{
		{"peanut types", "jeevan shikaram", "https://jeevan-shikaram.github.io"}, // TWA type
		{"twitter", "twitter, inc.", "https://mobile.twitter.com"},               // WebAPK type
	} {
		s.Logf("Launching %s from %s via omnibox", tc.title, tc.url)

		if err := conn.Navigate(ctx, tc.url); err != nil {
			s.Fatal("Failed to navigate to the url: ", err)
		}

		// Minimize the Play Store window to allow access to Install.
		window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return strings.Contains(w.Title, "Play Store")
		})
		if err != nil {
			s.Fatal("Failed to find the Play Store window: ", err)
		}
		if _, err := ash.SetWindowState(ctx, tconn, window.ID, ash.WMEventMinimize); err != nil {
			s.Fatal("Failed to minimize Play Store window: ", err)
		}

		// Locate and click on the omnibox install button.
		ui := uiauto.New(tconn)
		installButton := nodewith.ClassName("PwaInstallView").Role(role.Button)
		if err := ui.LeftClick(installButton)(ctx); err != nil {
			s.Fatal("Failed to left click omnibox install button: ", err)
		}

		if err := checkPlayStoreLaunched(ctx, d, tc.title, tc.publisher); err != nil {
			s.Fatal("Failed checking if play store launched: ", err)
		}
	}
}

// checkPlayStoreLaunched validates the Install button, app title and publisher are present.
func checkPlayStoreLaunched(ctx context.Context, d *androidui.Device, title, publisher string) error {
	// Check that the install button exists
	installButton := d.Object(androidui.ClassName("android.widget.Button"), androidui.TextMatches("(?i)install"), androidui.Enabled(true))
	if err := installButton.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrap(err, "failed finding install button")
	}

	// Check that the title exists
	appTitle := d.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)"+title), androidui.Enabled(true))
	if err := appTitle.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed finding %s text", title)
	}

	// Check that the publisher exists
	appPublisher := d.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)"+publisher), androidui.Enabled(true))
	if err := appPublisher.WaitForExists(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed finding %s text", publisher)
	}

	return nil
}
