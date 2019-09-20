// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/chromewpr"
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
		Vars:         []string{"platform.MemoryPressureModerate.maxTab"},
	})
}

func MemoryPressureModerate(ctx context.Context, s *testing.State) {
	var maxTab int

	// Check runtime flag maxTab to specify maximal tab count.
	if val, ok := s.Var("platform.MemoryPressureModerate.maxTab"); ok {
		s.Log("The number of maxTab is specified via runtime variable")

		tabs, err := strconv.Atoi(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryPressureModerate.maxTab: ", err)
		}
		maxTab = tabs
	} else {
		s.Log("Inferring the number of maxTab from the memory size")

		memInfo, err := kernelmeter.MemInfo()
		if err != nil {
			s.Fatal("Cannot obtain memory info: ", err)
		}

		// One tab consumes about 200 MB on average. Set a fixed tab count
		// to consume about 1.25x of the total memory.
		if memInfo.Total < kernelmeter.NewMemSizeMiB(2*1024) {
			maxTab = 13
		} else if memInfo.Total < kernelmeter.NewMemSizeMiB(4*1024) {
			maxTab = 25
		} else if memInfo.Total < kernelmeter.NewMemSizeMiB(8*1024) {
			maxTab = 50
		} else {
			maxTab = 100
		}
	}
	s.Log("Maximal tab count: ", maxTab)

	p := &mempressure.RunParameters{
		DormantCodePath:          s.DataPath(mempressure.DormantCode),
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
		MaxTabCount:              maxTab,
	}

	cp := &chromewpr.Params{
		WPRArchivePath: s.DataPath(mempressure.WPRArchiveName),
	}
	w, err := chromewpr.New(ctx, cp)
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer w.Close(ctx)

	mempressure.Run(ctx, s, w.Chrome, p)
}
