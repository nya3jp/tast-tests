// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/quickcheckcuj"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC02S2QuickCheckCUJ,
		Desc:         "Measures the smoothess of screen unlock and open an gmail thread",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc", "tablet_mode"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          cuj.LoginKeepState(),
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.cuj_wifissid",
			"ui.cuj_wifipassword",
		},
	})
}

func TC02S2QuickCheckCUJ(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(cuj.PreKeepData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard event writer: ", err)
	}
	defer kb.Close()

	defer func(ctx context.Context) {
		// Ensure that screen is unlocked even if the test fails.
		if st, err := lockscreen.GetState(ctx, tconn); err != nil {
			s.Error("Failed to get lockscreen state: ", err)
		} else if st.Locked {
			if err := kb.Type(ctx, password+"\n"); err != nil {
				s.Error("Failed to type password: ", err)
			}
		}
	}(ctx)

	const (
		lockTimeout     = 30 * time.Second
		goodAuthTimeout = 30 * time.Second
	)
	pv := quickcheckcuj.Run(ctx, s, func(ctx context.Context) error {
		const accel = "Search+L"
		s.Log("Locking screen via ", accel)
		if err := kb.Accel(ctx, accel); err != nil {
			return errors.Wrapf(err, "typing %v failed", accel)
		}
		s.Log("Waiting for Chrome to report that screen is locked")
		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
			return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
		}

		s.Log("Unlocking screen by typing password")
		if err = kb.Type(ctx, password+"\n"); err != nil {
			return errors.Wrap(err, "typing correct password failed")
		}
		s.Log("Waiting for Chrome to report that screen is unlocked")
		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
			return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
		}
		return nil
	})

	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
