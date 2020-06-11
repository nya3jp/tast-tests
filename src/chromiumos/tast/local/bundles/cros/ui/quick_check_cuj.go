// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ,
		Desc:         "Measures the smoothess of screen unlock and open an gmail thread",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Vars: []string{
			"ui.QuickCheckCUJ.username",
			"ui.QuickCheckCUJ.password",
		},
	})
}

func QuickCheckCUJ(ctx context.Context, s *testing.State) {
	const (
		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
		gmailTimeout    = 30 * time.Second
	)

	username := s.RequiredVar("ui.QuickCheckCUJ.username")
	password := s.RequiredVar("ui.QuickCheckCUJ.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(username, password, "gaia-id"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// lockState contains a subset of the state returned by chrome.autotestPrivate.loginStatus.
	type lockState struct {
		Locked bool `json:"isScreenLocked"`
		Ready  bool `json:"isReadyForPassword"`
	}

	// waitStatus repeatedly calls chrome.autotestPrivate.loginStatus and passes the returned
	// state to f until it returns true or timeout is reached. The last-seen state is returned.
	waitStatus := func(f func(st lockState) bool, timeout time.Duration) (lockState, error) {
		var st lockState
		err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
				return err
			} else if !f(st) {
				return errors.New("wrong status")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout})
		return st, err
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	const accel = "Search+L"
	s.Log("Locking screen via ", accel)
	if err := kb.Accel(ctx, accel); err != nil {
		s.Fatalf("Typing %v failed: %v", accel, err)
	}
	s.Log("Waiting for Chrome to report that screen is locked")
	if st, err := waitStatus(func(st lockState) bool { return st.Locked && st.Ready }, lockTimeout); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.startSmoothnessTracking)()`, nil); err != nil {
		s.Fatal("Failed to start display smoothness tracking: ", err)
	}
	start := time.Now()

	s.Log("Unlocking screen by typing password")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Typing correct password failed: ", err)
	}
	s.Log("Waiting for Chrome to report that screen is unlocked")
	if st, err := waitStatus(func(st lockState) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
	}

	conn, err := cr.NewConn(ctx, "https://www.gmail.com/")
	if err != nil {
		s.Fatal("Failed to open web: ", err)
	}
	defer conn.Close()

	s.Log("Opening the first email thread")
	row, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRow}, gmailTimeout)
	if err != nil {
		s.Fatal("Failed to find email thread row: ", err)
	}
	defer row.Release(ctx)

	if err := row.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click to open the email thread row: ", err)
	}

	if err := webutil.WaitForQuiescence(ctx, conn, gmailTimeout); err != nil {
		s.Fatal("Failed to wait for gmail to finish loading: ", err)
	}

	elapsed := time.Since(start)
	s.Log("Elapsed ms: ", elapsed.Milliseconds())

	var ds float64
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.stopSmoothnessTracking)()`, &ds); err != nil {
		s.Fatal("Failed to stop display smoothness tracking: ", err)
	}
	s.Log("Display smoothness: ", ds)

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "QuickCheckCUJ.DisplaySmoothness",
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, ds)

	pv.Set(perf.Metric{
		Name:      "QuickCheckCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(elapsed.Milliseconds()))

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
