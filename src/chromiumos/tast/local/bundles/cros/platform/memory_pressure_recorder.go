// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:     MemoryPressureRecorder,
		Desc:     "Record a WPR archive for platform.MemoryPressure",
		Contacts: []string{"semenzato@chromium.org", "sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"disabled", "informational"},
		Timeout:  30 * time.Minute,
		Data: []string{
			dormantCodeForRecorder,
		},
		SoftwareDeps: []string{"chrome_login"},
	})
}

const (
	dormantCodeForRecorder    = "memory_pressure_dormant.js"
	wprArchivePathForRecorder = "/usr/local/share/tast/archive.wprgo"
)

// MemoryPressureRecorder runs WPR in recording mode.
func MemoryPressureRecorder(ctx context.Context, s *testing.State) {
	p := &pressurizer.RunParameters{
		DormantCodePath: s.DataPath(dormantCodeForRecorder),
		WPRArchivePath:  wprArchivePathForRecorder,
		RecordPageSet:   true,
	}
	pressurizer.RunPressurizer(ctx, s, p)
}
