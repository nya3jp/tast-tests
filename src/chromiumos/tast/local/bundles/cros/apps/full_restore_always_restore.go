// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullRestoreAlwaysRestore,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test full restore always restore setting",
		Contacts: []string{
			"nancylingwang@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               browser.TypeLacros,
		}},
	})
}

func FullRestoreAlwaysRestore(ctx context.Context, s *testing.State) {
	const iterationCount = 7
	for i := 0; i < iterationCount; i++ {
		testing.ContextLogf(ctx, "Running: iteration %d/%d", i+1, iterationCount)

		openBrowser(ctx, s)
		err := restoreBrowser(ctx, s)
		if err != nil {
			s.Fatal("Failed to do full restore: ", err)
		}
	}
}

func openBrowser(ctx context.Context, s *testing.State) error {
	bt := s.Param().(browser.Type)

	// TODO(crbug.com/1318180): at the moment for Lacros, we're not getting SetUpWithNewChrome
	// close closure because when used it'd close all resources, including targets and wouldn't let
	// the session to proper restore later. As a short term workaround we're closing Lacros
	// resources using CloseResources fn instead, though ideally we want to use
	// SetUpWithNewChrome close closure when it's properly implemented.
	cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx,
		bt,
		lacrosfixt.NewConfig())
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect Test API")
	}

	// Open browser.
	// The opened browser is not closed before reboot so that it could be restored after reboot.
	conn, err := br.NewConn(ctx, "https://abc.xyz")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the restore URL")
	}
	defer conn.Close()

	// Open OS settings to set the 'Always restore' setting.
	if _, err = ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link)); err != nil {
		return errors.Wrap(err, "failed to launch Apps Settings")
	}

	if err := uiauto.Combine("set 'Always restore' Settings",
		uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
		uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
		return errors.Wrap(err, "failed to set 'Always restore' Settings")
	}

	// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
	// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
	// Therefore, sleep 3 seconds here.
	testing.Sleep(ctx, 3*time.Second)

	if bt == browser.TypeLacros {
		l, err := lacros.Connect(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to connect to lacros-chrome")
		}
		defer l.CloseResources(ctx)
	}

	return nil
}

func restoreBrowser(ctx context.Context, s *testing.State) error {
	opts := []chrome.Option{
		// Set not to clear the notification after restore.
		// By default, On startup is set to ask every time after reboot
		// and there is an alertdialog asking the user to select whether to restore or not.
		chrome.RemoveNotification(false),
		chrome.DisableFeatures("ChromeWhatsNewUI"),
		chrome.EnableRestoreTabs(),
		chrome.KeepState()}

	bt := s.Param().(browser.Type)
	if bt == browser.TypeLacros {
		lacrosOpts, err := lacrosfixt.NewConfig(lacrosfixt.ChromeOptions()).Opts()
		if err != nil {
			s.Fatal("Failed to get lacros options: ", err)
		}
		opts = append(opts, lacrosOpts...)
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect Test API")
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Confirm that the browser is restored.
	pollOptions := &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}
	if err := ash.WaitForCondition(ctx, tconn, ash.BrowserTitleMatch(bt, "Alphabet"), pollOptions); err != nil {
		s.Fatalf("Failed to wait for the window to be open, browser: %v, err: %v", bt, err)
	}

	// Confirm that the Settings app is restored.
	if err := uiauto.New(tconn).WaitUntilExists(ossettings.SearchBoxFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to restore the Settings app")
	}

	return nil
}
