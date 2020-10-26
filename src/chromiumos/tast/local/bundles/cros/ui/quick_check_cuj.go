// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ,
		Desc:         "Measures the smoothess of screen unlock and open an gmail thread",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          cuj.LoggedInToCUJUser(),
		Timeout:      4 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
		},
	})
}

func QuickCheckCUJ(ctx context.Context, s *testing.State) {
	const (
		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
		gmailTimeout    = 30 * time.Second
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.PreValue().(cuj.PreData).Chrome
	password := s.RequiredVar("ui.cuj_password")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	recorder, err := cuj.NewRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(closeCtx)

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
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
		s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
	}
	defer func() {
		// Ensure that screen is unlocked even if the test fails.
		if st, err := lockscreen.GetState(closeCtx, tconn); err != nil {
			s.Error("Failed to get lockscreen state: ", err)
		} else if st.Locked {
			if err := kb.Type(ctx, password+"\n"); err != nil {
				s.Error("Failed ot type password: ", err)
			}
		}
	}()

	var elapsed time.Duration
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		start := time.Now()

		s.Log("Unlocking screen by typing password")
		if err := kb.Type(ctx, password+"\n"); err != nil {
			return errors.Wrap(err, "typing correct password failed")
		}
		s.Log("Waiting for Chrome to report that screen is unlocked")
		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
			return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
		}

		conn, err := cr.NewConn(ctx, "https://www.gmail.com/")
		if err != nil {
			return errors.Wrap(err, "failed to open web")
		}
		defer conn.Close()

		s.Log("Opening the first email thread")
		row, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeRow}, gmailTimeout)
		if err != nil {
			return errors.Wrap(err, "failed to find email thread row")
		}
		defer row.Release(closeCtx)

		if err := row.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click to open the email thread row")
		}

		if err := webutil.WaitForQuiescence(ctx, conn, gmailTimeout); err != nil {
			return errors.Wrap(err, "failed to wait for gmail to finish loading")
		}

		elapsed = time.Since(start)
		s.Log("Elapsed ms: ", elapsed.Milliseconds())
		return nil
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "QuickCheckCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(elapsed.Milliseconds()))

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to collect the data from the recorder: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
