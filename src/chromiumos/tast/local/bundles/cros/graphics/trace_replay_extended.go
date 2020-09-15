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
		Func:         TraceReplayExtended,
		Desc:         "Repeatedly replay a 3D graphics trace file in Crostini VM for a fixed duration",
		Contacts:     []string{"chromeos-gfx@google.com", "ryanneph@google.com", "ddmail@google.com", "tutankhamen@google.com", "ihf@google.com"},
		Pre:          crostini.StartedByArtifact(),
		Data:         []string{crostini.ImageArtifact},
		SoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
		// We assign it to two different group in order to run it against pool:suite and pool:cros_av_analysis.
		Attr:    []string{"group:mainline", "informational", "group:graphics", "graphics_trace", "graphics_perbuild", "graphics_av_analysis"},
		Vars:    []string{"keepState"},
		Timeout: 45 * time.Minute,
		Params: []testing.Param{
			{
				Name: "glxgears_1minute",
				Val: comm.TestGroupConfig{
					Labels: []string{"short"},
					Repository: comm.RepositoryInfo{
						RootURL: "gs://chromiumos-test-assets-public/tast/cros/graphics/traces/repo",
						Version: 1,
					},
					ExtendedDurationInMinutes: 1,
				},
			},
		},
	})
}

// TraceReplayExtended replays a graphics trace repeatedly inside a crostini container.
func TraceReplayExtended(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	config := s.Param().(comm.TestGroupConfig)
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))
	if err := trace.RunTraceReplayTest(ctx, s.OutDir(), s.CloudStorage(), pre.Container, &config); err != nil {
		s.Fatal("Trace replay test failed: ", err)
	}
}
