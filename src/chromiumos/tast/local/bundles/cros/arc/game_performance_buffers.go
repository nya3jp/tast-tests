// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/gameperformance"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GamePerformanceBuffers,
		Desc:     "Captures set of performance metrics for buffers and upload it to the server",
		Contacts: []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-performance@google.com"},
		// TODO(http://b/140444033): Test is disabled until it can be fixed
		// Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"ArcGamePerformanceTest.apk"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func GamePerformanceBuffers(ctx context.Context, s *testing.State) {
	gameperformance.RunTest(ctx, s, "BuffersTest")
}
