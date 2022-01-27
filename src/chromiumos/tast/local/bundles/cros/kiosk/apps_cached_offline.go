// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.AutoLaunch(kioskmode.KioskAppAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	s.Log("Waiting for kiosk crx to be cached")
	if err := waitForCrxInCache(ctx, s, kioskmode.KioskAppID); err != nil {
		s.Fatal("Kiosk crx is not cached: ", err)
	}
	if err = cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome instance: ", err)
	}

	s.Log("Trying to launch kiosk app offline")
	restartAndLaunchKiosk := func(ctx context.Context) error {
		s.Log("Restarting ui")
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}

		reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
		if err != nil {
			s.Fatal("Failed to start log reader: ", err)
		}
		defer reader.Close()

		_, err = kiosk.StartNewChromeWithOptions(
			ctx,
			chrome.DMSPolicy(fdms.URL),
			chrome.NoLogin(),
			chrome.KeepState(),
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}

		if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
			s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
		}
		return nil
	}

	// Launch kiosk in offline mode.
	if err := network.ExecFuncOnChromeOffline(ctx, restartAndLaunchKiosk); err != nil {
		s.Fatal("Failed to launch kiosk app offline: ", err)
	}
}

// waitForCrxInCache waits for Kiosk crx to be available in cache.
func waitForCrxInCache(ctx context.Context, s *testing.State, id string) error {
	const crxCachePath = "/home/chronos/kiosk/crx/"
	ctx, st := timing.Start(ctx, "wait_crx_cache")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(crxCachePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.Wrap(err, "Kiosk crx cache does not exist yet")
			}
			return testing.PollBreak(errors.Wrap(err, "failed to list content of kiosk cache"))
		}

		for _, file := range files {
			if strings.HasPrefix(file.Name(), id) {
				s.Log("Found crx in cache: " + file.Name())
				return nil
			}
		}

		return errors.Wrap(err, "Kiosk crx cache does not have "+id)
	}, nil)
}
