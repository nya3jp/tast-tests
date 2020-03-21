// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/graphics/trace"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TraceReplayGlxgears,
		Desc:         "Replay glxgears trace file in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com", "tutankhamen@google.com", "ddmail@google.com", "ihf@google.com"},
		Attr:         []string{"group:graphics", "graphics_trace", "graphics_perbuild"},
		Pre:          crostini.StartedGPUEnabledBuster(),
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
	})
}

func TraceReplayGlxgears(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	testGroupConfig := comm.TestGroupConfig{
		Labels: []string{"short"},
		Repository: comm.RepositoryInfo{
			RootURL: "gs://chromiumos-test-assets-public/tast/cros/graphics/traces/repo",
			Version: 1,
		},
	}
	if err := trace.RunTraceReplayTest(ctx, s.OutDir(), s.CloudStorage(), pre.Container, &testGroupConfig); err != nil {
		s.Fatal("Trace replay test failed: ", err)
	}
}
