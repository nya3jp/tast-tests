// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/graphics/trace"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniTrace,
		Desc:         "Replay graphics trace in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com"},
		Data:         []string{crostini.ImageArtifact, "crostini_trace_glxgears.trace"},
		Pre:          crostini.StartedGPUEnabled(),
		SoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
		Params: []testing.Param{{
			Name:      "glxgears",
			Val:       map[string]string{"crostini_trace_glxgears": "glxgears"},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"group:mainline", "informational"},
		}},
	})
}

func CrostiniTrace(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	trace.RunTest(ctx, s, pre.Container, s.Param().(map[string]string))
}
