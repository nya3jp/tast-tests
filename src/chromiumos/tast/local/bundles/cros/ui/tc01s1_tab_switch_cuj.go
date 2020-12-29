// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
		Func:         TC01S1TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ, scrolling content with trackpad",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Minute,
		Pre:          wpr.RemoteReplayMode(),
		Vars: []string{
			"mute",
			"ui.cuj_username",
			"ui.cuj_password",
			"wpr_http_addr",
			"wpr_https_addr",
			"perf_level",
		},
	})
}

// TC01S1TabSwitchCUJ measures the performance of tab-switching CUJ, scrolling content with trackpad
// It use package tabswitchcuj to do testing, except the setup of precondition is different.
// In order to ensure wpr service won't affect DUT's performance, this test case use a remote wpr service,
// all traffic will be redirected to remote wpr service,
// thus, remote side will determinate wpr is going to record or replay, not by this test case
//
// Steps to update the test (by using a remote wpr service):
//  1. make changes (package tabswitchcuj)
//  2. manually run wpr (record mode) on remote server
//  3. run this test
//  4. manually terminate wpr to output a record file on remote server
//  5. change configureation (mode and the record file to use) of wpr on remote server
//  6. manually run wpr (replay mode) on remote server
//  7. run this test
func TC01S1TabSwitchCUJ(ctx context.Context, s *testing.State) {
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
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

	level := tabswitchcuj.Basic
	pl := s.RequiredVar("perf_level")
	switch pl {
	default:
	case "Basic":
		level = tabswitchcuj.Basic
	case "Plus":
		level = tabswitchcuj.Plus
	case "Premium":
		level = tabswitchcuj.Premium
	}

	var (
		x      = pad.Width() / 2
		ystart = pad.Height() / 4
		yend   = pad.Height() / 4 * 3
	)

	// swipe the page down
	doubleSwipeDown := func(ctx context.Context) error {
		if err := touch.DoubleSwipe(ctx, x, ystart, x, yend, 8, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := touch.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// swipe the page up
	doubleSwipeUp := func(ctx context.Context) error {
		if err = touch.DoubleSwipe(ctx, x, yend, x, ystart, 8, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := touch.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	actions := []func(ctx context.Context) error{
		doubleSwipeDown,
		doubleSwipeUp,
		doubleSwipeUp,
	}

	tabswitchcuj.Run(ctx, s, cr, tabswitchcuj.TestOption{TestLevel: level, TabActions: actions})
}
