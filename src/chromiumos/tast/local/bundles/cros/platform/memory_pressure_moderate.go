// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryPressureModerate,
		Desc:     "Measure tab switching performance under moderate memory pressure",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  180 * time.Minute,
		Data: []string{
			mempressure.CompressibleData,
			mempressure.DormantCode,
			mempressure.WPRArchiveName,
		},
		SoftwareDeps: []string{"chrome"},
	})
}

// MemoryPressureModerate is the main test function.
func MemoryPressureModerate(ctx context.Context, s *testing.State) {
	memInfo, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Cannot obtain memory info: ", err)
	}

	// One tab consumes about 200 MB on average. Set a fixed tab count
	// to consume about 1.25x of the total memory.
	var maxTab int
	if memInfo.Total < kernelmeter.NewMemSizeMiB(2*1024) {
		maxTab = 13
	} else if memInfo.Total < kernelmeter.NewMemSizeMiB(4*1024) {
		maxTab = 25
	} else if memInfo.Total < kernelmeter.NewMemSizeMiB(8*1024) {
		maxTab = 50
	} else {
		maxTab = 100
	}

	p := &mempressure.RunParameters{
		DormantCodePath:          s.DataPath(mempressure.DormantCode),
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
		WPRArchivePath:           s.DataPath(mempressure.WPRArchiveName),
		MaxTabCount:              maxTab,
	}
	mempressure.Run(ctx, s, p)
}
