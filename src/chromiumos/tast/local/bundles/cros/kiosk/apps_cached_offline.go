// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AppsCachedOffline,
		Desc: "Checks if Kiosk apps can be cached and launched offline",
		Contacts: []string{
			"yixie@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.KioskAutoLaunchCleanup,
	})
}

func AppsCachedOffline(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	kiosk, _, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.AutoLaunch(kioskmode.KioskAppAccountID),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	s.Log("Waiting for Kiosk crx to be cached")
	if err := kioskmode.WaitForCrxInCache(ctx, kioskmode.KioskAppID); err != nil {
		s.Fatal("Kiosk crx is not cached: ", err)
	}

	s.Log("Trying to launch Kiosk app offline")
	restartAndLaunchKiosk := func(ctx context.Context) error {
		reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
		if err != nil {
			s.Fatal("Failed to start log reader: ", err)
		}
		defer reader.Close()

		_, err = kiosk.RestartChromeWithOptions(
			ctx,
			chrome.DMSPolicy(fdms.URL),
			chrome.NoLogin(),
			chrome.KeepState(),
		)
		if err != nil {
			s.Fatal("Failed to restart Chrome: ", err)
		}

		if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
			s.Fatal("Kiosk is not started after restarting Chrome: ", err)
		}

		return nil
	}

	// Launch kiosk in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, restartAndLaunchKiosk); err != nil {
		s.Fatal("Failed to launch kiosk app offline: ", err)
	}
}
