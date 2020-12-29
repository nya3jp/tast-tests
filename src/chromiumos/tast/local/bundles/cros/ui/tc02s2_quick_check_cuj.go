// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/quickcheckcuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC02S2QuickCheckCUJ,
		Desc:         "Measures the system performance after unlocking the screen by checking common apps",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInToCUJUserKeepState",
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_password",
			"ui.cuj_wifissid",
			"ui.cuj_wifipassword",
			"perf_level",
		},
	})
}

// TC02S2QuickCheckCUJ executes the following logic:
// Open locked device device and login, and connect to a remembered wifi network. Open common apps
// and do online browsing.
func TC02S2QuickCheckCUJ(ctx context.Context, s *testing.State) {
	tabletMode := false
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	pv := quickcheckcuj.Run(ctx, s, cr, quickcheckcuj.Lock, tabletMode)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
