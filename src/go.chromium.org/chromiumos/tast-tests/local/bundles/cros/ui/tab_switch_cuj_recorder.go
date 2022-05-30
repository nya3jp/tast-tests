// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"path/filepath"
	"time"

	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/ui/tabswitchcuj"
	"go.chromium.org/chromiumos/tast-tests/local/wpr"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJRecorder,
		LacrosStatus: testing.LacrosVariantUnknown,
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
