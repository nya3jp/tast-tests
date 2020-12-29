// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC01S1TabSwitchCUJTrackpad,
		Desc:         "Measures the performance of tab-switching CUJ, scrolling content with trackpad",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
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

func TC01S1TabSwitchCUJTrackpad(ctx context.Context, s *testing.State) {
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		cr = s.PreValue().(cuj.PreKeepData).Chrome
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Move the mouse cursor to the center so the scrolling will be effected
	// on the web page
	screen, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}
	if err = mouse.Move(ctx, tconn, screen.Bounds.CenterPoint(), time.Second); err != nil {
		s.Fatal("Failed to move the mouse cursor to the center: ", err)
	}

	pad, err := input.VirtualTrackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create trackpad event writer: ", err)
	}
	defer pad.Close()

	touch, err := pad.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create trackpad singletouch writer: ", err)
	}
	defer touch.Close()

	var (
		x      = pad.Width() / 2
		ystart = pad.Height() / 4
		yend   = pad.Height() / 4 * 3
	)
	tabswitchcuj.Run(ctx, s, func(ctx context.Context) error {
		defer touch.End()
		return touch.DoubleSwipe(ctx, x, ystart, x, yend, 8, 500*time.Millisecond)
	})
}
