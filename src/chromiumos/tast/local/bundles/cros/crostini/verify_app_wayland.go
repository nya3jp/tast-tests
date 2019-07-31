// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/verifyapp"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyAppWayland,
		Desc:         "Runs a Wayland crostini application from the terminal and verifies that it renders",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func VerifyAppWayland(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	verifyapp.RunTest(ctx, s, pre.Chrome, pre.Container, "wayland", crostini.WaylandDemoAppPath, colorcmp.RGB(0x33, 0x88, 0xdd))
}
