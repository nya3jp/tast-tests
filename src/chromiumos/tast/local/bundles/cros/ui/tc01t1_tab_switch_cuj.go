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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC01T1TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ, scrolling content with touchscreen",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.TouchScreen()),
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

// TC01T1TabSwitchCUJ measures the performance of tab-switching CUJ, scrolling content with touchscreen.
//
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
func TC01T1TabSwitchCUJ(ctx context.Context, s *testing.State) {
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
		x      = screen.Width() / 2
		ystart = screen.Height() / 4 * 3 // 75% of screen height
		yend   = screen.Height() / 4     // 25% of screen height
	)

	swipeDown := func(ctx context.Context) error {
		if err := touch.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := touch.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	actions := []func(ctx context.Context) error{
		swipeDown,
	}

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}

	tabswitchcuj.Run(ctx, s, cr, tabswitchcuj.TestOption{TestLevel: level, TabActions: actions})
}
