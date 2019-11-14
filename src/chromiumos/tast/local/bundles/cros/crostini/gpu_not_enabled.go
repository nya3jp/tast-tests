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
		Func:         GPUNotEnabled,
		Desc:         "Ensures that when crostini boots without the gpu flag the GPU is not enabled",
		Contacts:     []string{"hollingum@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Pre:       crostini.StartedByArtifact(),
				Name:      "artifact",
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name:    "download",
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "buster",
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
		},
	})
}

func GPUNotEnabled(ctx context.Context, s *testing.State) {
	// In tast, we do not initialize the VM the normal way, so even though the GPU is enabled by default on some boards, this precondition will still have the GPU disabled.
	gpuenabled.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container, "llvmpipe")
}
