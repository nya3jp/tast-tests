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
		Desc:         "Run tab-switching CUJ test in chromewpr recording mode",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "hc.tsai@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Minute,
		Vars: []string{
			"mute",
			"wpr_http_addr",
			"wpr_https_addr",
		},
		// if the wpr service is host on DUT, it has been found that some website cannot be accessed (e.g., reddit),
		// hence, here using a remote wpr service, which the configuration of record/replay mode needs to manually switch on remote side
		Pre: wpr.RemoteReplayMode(),
	})
}

// TabSwitchCUJRecorder2 run tab-switching CUJ test in chromewpr recording mode. It will
// record the premium scenario, which can be used for basic and plus testing as well.
//
// Steps to do record:
//  1. manually run wpr in record mode on remote server
//  2. run this test
//  3. manually terminate wpr to output a record file on remote server
//  4. check remote wpr configureation to find the record file
func TabSwitchCUJRecorder2(ctx context.Context, s *testing.State) {
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}
	// is the dut tablet or not shouldn't affect to recording web content
	tabswitchcuj.Run2(ctx, s, cr, tabswitchcuj.Record, false)
}
