// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/graphics/trace"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniTraceGlxgears,
		Desc:         "Replay graphics trace in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{"crostini_trace_glxgears_glxgears.trace"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniTraceGlxgears(ctx context.Context, s *testing.State) {
	var traceNameMap = make(map[string]string)
	traceNameMap["crostini_trace_glxgears_glxgears.trace"] = "glxgears"
	trace.RunTest(ctx, s, traceNameMap)
}
