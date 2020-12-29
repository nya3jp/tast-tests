// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ2,
		Desc:         "Measures the performance of tab-switching CUJ, scrolling content with trackpad",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          wpr.RemoteReplayMode(),
		Vars: []string{
			"mute",
			"mode", // Expecting "tablet" or "clamshell".
			"ui.cuj_username",
			"ui.cuj_password",
			"wpr_http_addr",
			"wpr_https_addr",
		},
		Params: []testing.Param{
			{
				Name:    "basic",
				Timeout: 30 * time.Minute,
				Val:     tabswitchcuj.Basic,
			}, {
				Name:    "plus",
				Timeout: 35 * time.Minute,
				Val:     tabswitchcuj.Plus,
			}, {
				Name:    "premium",
				Timeout: 40 * time.Minute,
				Val:     tabswitchcuj.Premium,
			},
		},
	})
}

// TabSwitchCUJ2 measures the performance of tab-switching CUJ.
//
// WPR server should be running in a remote server. TabSwitchCUJRecorder2 case can be used to record
// WPR content for this test in the remote server.
func TabSwitchCUJ2(ctx context.Context, s *testing.State) {
	level := s.Param().(tabswitchcuj.Level)

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabletMode, cleanup, err := cuj.RunTabletMode(ctx, s, tconn)
	if err != nil {
		s.Fatal("Failed to determine the screen mode: ", err)
	}
	defer cleanup(ctx)

	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	tabswitchcuj.Run2(ctx, s, cr, level, tabletMode)
}
