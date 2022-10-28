// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory/mempressure"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryPressureRecorder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Record a WPR archive for platform.MemoryPressure",
		Contacts:     []string{"bgeffon@chromium.org", "vovoy@chromium.org", "chromeos-memory@google.com"},
		Timeout:      60 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Pre:          wpr.RecordMode("/tmp/archive.wprgo"),
	})
}

// MemoryPressureRecorder runs WPR in recording mode.
func MemoryPressureRecorder(ctx context.Context, s *testing.State) {
	p := &mempressure.RunParameters{
		Mode: wpr.Record,
	}

	if err := mempressure.Run(ctx, s.OutDir(), s.PreValue().(*chrome.Chrome), nil, p); err != nil {
		s.Fatal("Run failed: ", err)
	}
}
