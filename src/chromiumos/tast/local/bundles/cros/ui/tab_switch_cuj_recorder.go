// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/tabswitchcuj"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJRecorder,
		LacrosStatus: testing.LacrosVariantUnneeded, // used to record all web traffic via wpr so that later TabSwitchCUJ could run without really talking to real sites
		Desc:         "Run tab-switching CUJ test in chromewpr recording mode",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"mute"},
		Pre:          wpr.RecordMode(filepath.Join("/tmp", tabswitchcuj.WPRArchiveName)),
	})
}

func TabSwitchCUJRecorder(ctx context.Context, s *testing.State) {
	tabswitchcuj.Run(ctx, s)
}
