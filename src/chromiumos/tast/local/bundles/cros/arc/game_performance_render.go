// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/gameperformance"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamePerformanceRender,
		Desc:         "Captures set of performance metrics for the render and upload it to the server",
		Contacts:     []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-performance@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{"ArcGamePerformanceTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      30 * time.Minute,
	})
}

func GamePerformanceRender(ctx context.Context, s *testing.State) {
	gameperformance.RunTest(ctx, s, "RenderTest")
}
