// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
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
			mempressure.WPRArchiveName,
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"platform.MemoryPressureModerate.maxTab"},
		Pre:          wpr.ReplayMode(mempressure.WPRArchiveName),
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
		PageFilePath:             s.DataPath(mempressure.CompressibleData),
		PageFileCompressionRatio: 0.40,
		MaxTabCount:              maxTab,
	}

	mempressure.Run(ctx, s, s.PreValue().(*chrome.Chrome), p)
}
