// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/verifyapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyAppX11,
		Desc:         "Runs an X11 crostini application from the terminal and verifies that it renders",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				ExtraSoftwareDeps: []string{"crostini_stable"},
			},
			{
				Name:              "artifact_unstable",
				ExtraSoftwareDeps: []string{"crostini_unstable"},
			},
		},
	})
}

func VerifyAppX11(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	verifyapp.RunTest(ctx, s, pre.Chrome, pre.Container, crostini.X11DemoConfig())
}
