// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryPressure,
		Desc:     "Create memory pressure and collect various measurements from Chrome and from the kernel",
		Contacts: []string{"semenzato@chromium.org", "sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  180 * time.Minute,
		Data: []string{
			mempressure.CompressibleData,
			mempressure.WPRArchiveName,
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          wpr.ReplayMode(mempressure.WPRArchiveName),
	})
}

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	p := &mempressure.RunParameters{
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
	}

	mempressure.Run(ctx, s, s.PreValue().(*chrome.Chrome), p)
}
