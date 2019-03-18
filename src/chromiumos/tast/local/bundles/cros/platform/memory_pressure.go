// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/pressurizer"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryPressure,
		Desc:     "Create memory pressure and collect various measurements from Chrome and from the kernel",
		Contacts: []string{"semenzato@chromium.org", "sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:  30 * time.Minute,
		Data: []string{
			compressibleData,
			dormantCode,
			preallocatorScript,
			wprArchiveName,
		},
		SoftwareDeps: []string{"chrome_login"},
	})
}

const (
	// compressibleData is a file containing compressible data for preallocation.
	compressibleData = "memory_pressure_page.lzo.40"
	// dormantCode is JS code that detects the end of a page load.
	dormantCode = "memory_pressure_dormant.js"
	// preallocatorScript is a shell script that preallocates memory.
	preallocatorScript = "memory_pressure_preallocator.sh"
	// wprArchiveName is the external file name for the wpr archive.
	wprArchiveName = "memory_pressure_mixed_sites.wprgo"
)

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	p := &pressurizer.RunParameters{
		DormantCodePath:          s.DataPath(dormantCode),
		PageFilePath:             s.DataPath(compressibleData),
		PageFileCompressionRatio: 0.40,
		PreallocatorPath:         s.DataPath(preallocatorScript),
		WPRArchivePath:           s.DataPath(wprArchiveName),
	}
	pressurizer.RunPressurizer(ctx, s, p)
}
