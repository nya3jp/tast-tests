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
		Func:         GamePerformanceRenderUnderLoad,
		Desc:         "Captures set of performance metrics for the render under the load and upload it to the server. This test takes long time so use it for manual run only. See also GamePerformanceRender",
		Contacts:     []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-performance@google.com"},
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcGamePerformanceTest.apk"},
		Timeout:      1 * time.Hour,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

func GamePerformanceRenderUnderLoad(ctx context.Context, s *testing.State) {
	gameperformance.RunTest(ctx, s, "RenderUnderLoadTest")
}
