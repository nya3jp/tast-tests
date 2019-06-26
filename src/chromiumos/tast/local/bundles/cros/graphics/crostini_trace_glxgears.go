// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/graphics/trace"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniTraceGlxgears,
		Desc:         "Replay graphics trace in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{"crostini_trace_glxgears.trace"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
	})
}

func CrostiniTraceGlxgears(ctx context.Context, s *testing.State) {
	trace.RunTest(ctx, s, map[string]string{"crostini_trace_glxgears.trace": "glxgears"})
}
