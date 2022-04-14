// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJRecorder2,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Run tab-switching CUJ test in chromewpr recording mode",
		Contacts:     []string{"abergman@google.com", "tclaiborne@chromium.org", "xliu@cienet.com", "alfredyu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Minute,
		Params: []testing.Param{
			{
				Name: "local",
				Val:  "local",
				Pre:  wpr.RecordMode("/tmp/archive.wprgo"),
			}, {
				Name: "remote",
				Val:  "remote",
				// Remote WPR proxy is set up manually. Use RemoteReplayMode() to
				// get remote WPR address and start chrome accordingly.
				Pre: wpr.RemoteReplayMode(),
			},
		},
		Vars: []string{
			"ui.cuj_mute",
			// The following vars are only used by remote mode.
			"ui.wpr_http_addr",
			"ui.wpr_https_addr",
		},
	})
}

// TabSwitchCUJRecorder2 runs tab-switching CUJ test in chrome wpr recording mode. It will
// record the premium scenario, which can be used for basic and plus testing as well.
//
// The test can either do recording on a DUT local WPR server, or a remote WPR server.
// Local WPR server will be set up automatically by the preconditon.
// Steps to do remote recording:
//  1. Manually run wpr in record mode on a remote server.
//  2. Run this test.
//  3. Manually terminate wpr to output a record file on remote server.
//  4. Check remote wpr configureation to find the record file
func TabSwitchCUJRecorder2(ctx context.Context, s *testing.State) {
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}
	// is the dut tablet or not shouldn't affect to recording web content
	// Currently recorder is supported for ash-Chrome only. We call Run2() with lFixtVal as nil.
	// If support of lacros is needed, we need to enhance the test to pass lacrosFixtValue.
	tabswitchcuj.Run2(ctx, s, cr, tabswitchcuj.Record, false, nil)
}
