// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type accountTypeParam struct {
	unicorn bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableArc,
		Desc:         "Verify PlayStore can be turned off in Settings ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val: accountTypeParam{
				unicorn: false,
			},
		}, {
			Name:              "unicorn",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: accountTypeParam{
				unicorn: true,
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: accountTypeParam{
				unicorn: false,
			},
		}, {
			Name:              "unicorn_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: accountTypeParam{
				unicorn: true,
			},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.parentUser", "arc.parentPassword", "arc.childUser", "arc.childPassword"},
	})
}

func DisableArc(ctx context.Context, s *testing.State) {

	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")
	childUser := s.RequiredVar("arc.childUser")
	childPass := s.RequiredVar("arc.childPassword")

	var cr *chrome.Chrome
	var err error

	accountType := s.Param().(accountTypeParam)
	if accountType.unicorn {
		cr, err = chrome.New(
			ctx,
			chrome.GAIALogin(chrome.Creds{
				User:       childUser,
				Pass:       childPass,
				ParentUser: parentUser,
				ParentPass: parentPass,
			}),
			chrome.ARCSupported())
	} else {
		cr, err = chrome.New(
			ctx,
			chrome.GAIALogin(chrome.Creds{
				User: parentUser,
				Pass: parentPass,
			}),
			chrome.ARCSupported(),
			chrome.ExtraArgs(arc.DisableSyncFlags()...))
	}

	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to PlayStore and Close
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	// Setup screen recording and saving on error.
	// TODO(b/178232263): remove this once the test is fixed.
	s.Log("Starting screen recording")
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}
	defer func() {
		screenRecorder.Stop(ctx)
		if s.HasError() {
			s.Log(ctx, "Saving screen record to %s", s.OutDir())
			if err := screenRecorder.SaveInString(ctx, filepath.Join(s.OutDir(), "screen_record.txt")); err != nil {
				s.Fatal("Failed to save screen record in string: ", err)
			}
			if err := screenRecorder.SaveInBytes(ctx, filepath.Join(s.OutDir(), "screen_record.webm")); err != nil {
				s.Fatal("Failed to save screen record in bytes: ", err)
			}
		}
		screenRecorder.Release(ctx)
	}()
	screenRecorder.Start(ctx, tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Turn Play Store Off from Settings")
	if err := turnOffPlayStore(ctx, tconn); err != nil {
		s.Fatal("Failed to Turn Off Play Store: ", err)
	}

	s.Log("Verify Play Store is Off")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playStoreState, err := optin.GetPlayStoreState(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get some playstore state")
		}
		if playStoreState["enabled"] == true {
			return errors.New("Playstore is On Still")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to verify Play Store is off: ", err)
	}

}

func turnOffPlayStore(ctx context.Context, tconn *chrome.TestConn) error {
	// Navigate to Android Settings.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to launch the Settings app")
	}

	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	return uiauto.Combine("turn off Play Store",
		ui.LeftClickUntil(nodewith.Name("Apps").Role(role.Heading), ui.Exists(playStoreButton)),
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Remove Google Play Store").Role(role.Button)),
		ui.LeftClick(nodewith.Name("Remove Android apps").Role(role.Button)),
	)(ctx)
}
