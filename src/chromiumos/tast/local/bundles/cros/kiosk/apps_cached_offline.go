// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppsCachedOffline,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks if Kiosk apps can be cached and launched offline",
		Contacts: []string{
			"yixie@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "ash",
			Val:  chrome.ExtraArgs(""),
		}, {
			Name:              "lacros",
			Val:               chrome.ExtraArgs("--enable-features=LacrosSupport,ChromeKioskEnableLacros", "--lacros-availability-ignore"),
			ExtraSoftwareDeps: []string{"lacros"},
		}},
		Fixture: fixture.KioskAutoLaunchCleanup,
		Timeout: 5 * time.Minute, // Starting Kiosk twice requires longer timeout.
	})
}

func AppsCachedOffline(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	chromeOptions := s.Param().(chrome.Option)
	kiosk, _, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.AutoLaunch(kioskmode.KioskAppAccountID),
		kioskmode.ExtraChromeOptions(
			chromeOptions,
		),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	s.Log("Waiting for Kiosk crx to be cached")
	if err := kioskmode.WaitForCrxInCache(ctx, kioskmode.KioskAppID); err != nil {
		s.Fatal("Kiosk crx is not cached: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	technologies, err := manager.GetEnabledTechnologies(ctx)
	if err != nil {
		s.Fatal("Failed to get enabled technologies: ", err)
	}

	// Reserve 5 seconds to restore connection settings.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var enableFuncs []func(context.Context)
	defer func(cleanupCtx context.Context) {
		for _, enableFunc := range enableFuncs {
			enableFunc(cleanupCtx)
		}
	}(cleanupCtx)

	// Disable all connection technologies
	for _, t := range technologies {
		enableFunc, err := manager.DisableTechnologyForTesting(ctx, t)
		if err != nil {
			s.Fatalf("Failed to disable %v technology: %s", t, err)
		}
		testing.ContextLog(ctx, "Disabled connection technology: ", t)
		enableFuncs = append(enableFuncs, enableFunc)
	}

	s.Log("Trying to launch Kiosk app offline")

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
		chromeOptions,
	)
	if err != nil {
		s.Fatal("Failed to restart Chrome: ", err)
	}

	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		s.Fatal("Kiosk is not started after restarting Chrome: ", err)
	}
}
