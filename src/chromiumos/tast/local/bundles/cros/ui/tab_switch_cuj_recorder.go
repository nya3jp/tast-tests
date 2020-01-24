// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Desc:         "Run tab-switching CUJ test in chromewpr recording mode",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"mute"},
		Pre:          wpr.RecordMode(filepath.Join("/tmp", tabswitchcuj.WPRArchiveName)),
	})
}

func TabSwitchCUJRecorder(ctx context.Context, s *testing.State) {
	tabswitchcuj.Run(ctx, s)
}
