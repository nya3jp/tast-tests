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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickCheckCUJ,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the smoothess of screen unlock and open an gmail thread",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
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
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	var cs ash.ConnSource
	var cr *chrome.Chrome

	if bt == browser.TypeAsh {
		cr = s.FixtValue().(cuj.FixtureData).Chrome
		cs = cr
	} else {
		var err error
		var l *lacros.Lacros
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), bt)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacros(closeCtx, l)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	password := cr.Creds().Pass

	configs := []cuj.MetricConfig{
		cuj.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter, []int64{50, 80}),
		cuj.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter, []int64{0, 3}),
	}
	recorder, err := cuj.NewRecorder(ctx, cr, nil, configs...)
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
			if err := kb.Type(closeCtx, password+"\n"); err != nil {
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

		conn, err := cs.NewConn(ctx, "https://www.gmail.com/")
		if err != nil {
			return errors.Wrap(err, "failed to open web")
		}
		defer conn.Close()

		s.Log("Opening the first email thread")
		firstRow := nodewith.Role(role.Row).First()
		ac := uiauto.New(tconn)
		if err := ac.LeftClick(firstRow)(ctx); err != nil {
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
