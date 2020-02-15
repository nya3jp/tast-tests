// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         TraceReplayGlxgears,
		Desc:         "Replay glxgears trace file in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com", "tutankhamen@google.com", "ddmail@google.com", "ihf@google.com"},
		Attr:         []string{"group:graphics", "graphics_trace", "graphics_perbuild"},
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedGPUEnabledBuster(),
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
	})
}

func TraceReplayGlxgears(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	entries := []*trace.TestEntryConfig{
		{
			Name: "glxgears",
			StorageFile: trace.FileInfo{
				GSURL:     "gs://chromiumos-test-assets-public/tast/cros/graphics/traces/glxgears.trace.bz2",
				Size:      61066,
				SHA256Sum: "1b36209dc466b3ebaea84295d5a5bc5e9df0b037215379dae43518e9f27fd2f3",
				MD5Sum:    "e59e9f99ab035399c7fabd53e3e08829",
			},
			TestSettings: trace.TestSettings{
				RepeatCount:    1,
				CoolDownIntSec: 0,
			},
		},
	}
	if err := trace.RunTraceReplayTest(ctx, s.OutDir(), s.CloudStorage(), pre.Container, entries); err != nil {
		s.Fatal("Trace replay test failed: ", err)
	}
}
