// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"syscall"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosproc"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LacrosRestartOnCrash,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that lacros does properly restart in webkiosk mode",
		Contacts: []string{
			"zubeil@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
	})
}

func LacrosRestartOnCrash(ctx context.Context, s *testing.State) {

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore")
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.ExtraChromeOptions(chromeOptions),
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
	)

	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer kiosk.Close(ctx)

	testing.ContextLog(ctx, "Waiting for API connection")
	testing.ContextLog(ctx, "Chrome log dir: ", cr.LogFilename())
	testing.Sleep(ctx, 10*time.Second)

	testing.ContextLog(ctx, "Killing lacros")
	proc, err := lacrosproc.Root()
	if err != nil {
		s.Fatal("Failed to get lacros proc: ", err)
	}

	if err := proc.SendSignalWithContext(ctx, syscall.SIGSEGV); err != nil {
		s.Fatal("Failed to crash chrome: ", err)
	}

	testing.ContextLog(ctx, "Lacros was killed")
	testing.Sleep(ctx, 200*time.Second)

}
