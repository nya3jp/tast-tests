// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC01S2TabSwitchCUJTouchscreen,
		Desc:         "Measures the performance of tab-switching CUJ, scrolling content with touchscreen",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.TouchScreen()),
		Timeout:      30 * time.Minute,
		Vars: []string{
			"mute",
			"ui.cuj_username",
			"ui.cuj_password",
			"wpr_http_addr",
			"wpr_https_addr",
		},
		Params: []testing.Param{{
			Name:              "online",
			ExtraSoftwareDeps: []string{"arc"},
			Pre:               cuj.LoginKeepState(),
		}, {
			Name: "replay",
			Pre:  wpr.Remote(),
		}},
	})
}

func TC01S2TabSwitchCUJTouchscreen(ctx context.Context, s *testing.State) {
	screen, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touchscreen event writer: ", err)
	}
	defer screen.Close()

	touch, err := screen.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create touchscreen singletouch writer: ", err)
	}
	defer touch.Close()

	var (
		x      = screen.Width() / 2
		ystart = screen.Height() / 4 * 3 // 75% of screen height
		yend   = screen.Height() / 4     // 25% of screen height
	)
	tabswitchcuj.Run(ctx, s, func(ctx context.Context) error {
		defer touch.End()
		return touch.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond)
	})
}
