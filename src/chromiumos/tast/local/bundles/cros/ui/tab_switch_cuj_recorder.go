// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJRecorder,
		Desc:         "Run tab-switching CUJ test in chromewpr recording mode",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com", "hc.tsai@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      30 * time.Minute,
		Vars:         []string{"mute"},
		Pre:          wpr.RecordMode(filepath.Join("/tmp", tabswitchcuj.WPRArchiveName)),
	})
}

// TabSwitchCUJRecorder run tab-switching CUJ test in chromewpr recording mode
func TabSwitchCUJRecorder(ctx context.Context, s *testing.State) {
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		s.Fatal("Failed to connect to Chrome")
	}
	tabswitchcuj.Run(ctx, s, cr, tabswitchcuj.TestOption{TestLevel: tabswitchcuj.Record, TabActions: []func(ctx context.Context) error{}})
}
