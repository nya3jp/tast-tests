// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/network/firewall"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/network"
	local_firewall "chromiumos/tast/local/network/firewall"
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

	s.Log("Trying to launch Kiosk app offline")
	restartAndLaunchKiosk := func(ctx context.Context) error {
		reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
		if err != nil {
			return errors.Wrap(err, "failed to start log reader")
		}
		defer reader.Close()

		// Additionally block access to FakeDMS to make Chrome think it's offline.
		firewallRunner := local_firewall.NewLocalRunner()
		fdmsURL, err := url.Parse(fdms.URL)
		if err != nil {
			return errors.Wrap(err, "failed to parse FakeDMS URL")
		}
		fdmsPort, err := strconv.Atoi(fdmsURL.Port())
		if err != nil {
			return errors.Wrap(err, "failed to parse port from FakeDMS URL")
		}

		commonRuleArgs := []firewall.RuleOption{
			firewall.OptionProto(firewall.L4ProtoTCP),
			firewall.OptionUIDOwner("chronos"),
			firewall.OptionDPort(fdmsPort),
			firewall.OptionJumpTarget(firewall.TargetDrop),
		}
		ruleArgs := []firewall.RuleOption{firewall.OptionAppendRule(firewall.OutputChain)}
		ruleArgs = append(ruleArgs, commonRuleArgs...)
		if err := firewallRunner.ExecuteCommand(ctx, ruleArgs...); err != nil {
			return errors.Wrap(err, "failed to block access to FakeDMS")
		}

		// Reserve 3 seconds to resume firewall settings.
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
		defer cancel()

		defer func(cleanupCtx context.Context) {
			ruleArgs := []firewall.RuleOption{firewall.OptionDeleteRule(firewall.OutputChain)}
			ruleArgs = append(ruleArgs, commonRuleArgs...)
			if err := firewallRunner.ExecuteCommand(ctx, ruleArgs...); err != nil {
				s.Fatal("Failed to restore access to FakeDMS: ", err)
			}
		}(cleanupCtx)

		_, err = kiosk.RestartChromeWithOptions(
			ctx,
			chrome.DMSPolicy(fdms.URL),
			chrome.NoLogin(),
			chrome.KeepState(),
			chromeOptions,
		)
		if err != nil {
			return errors.Wrap(err, "failed to restart Chrome")
		}

		if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
			return errors.Wrap(err, "kiosk is not started after restarting Chrome")
		}

		return nil
	}

	// Launch kiosk in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, restartAndLaunchKiosk); err != nil {
		s.Fatal("Failed to launch kiosk app offline: ", err)
	}
}
