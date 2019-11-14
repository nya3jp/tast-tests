// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/gpuenabled"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GPUEnabled,
		Desc:     "Ensures that when crostini boots with the GPU enabled that it really is accessible as a device in the container",
		Contacts: []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:      "artifact",
				Pre:       crostini.StartedGPUEnabledArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name:    "download",
				Pre:     crostini.StartedGPUEnabledDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "buster",
				Pre:     crostini.StartedGPUEnabledBuster(),
				Timeout: 10 * time.Minute,
			},
		},
		SoftwareDeps: []string{"chrome", "vm_host", "crosvm_gpu"},
	})
}

func GPUEnabled(ctx context.Context, s *testing.State) {
	gpuenabled.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container, "virgl")
}
