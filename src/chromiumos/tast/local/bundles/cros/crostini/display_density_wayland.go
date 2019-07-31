// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/displaydensity"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayDensityWayland,
		Desc:         "Runs a Wayland crostini application from the terminal in high/low DPI modes and compares sizes",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func DisplayDensityWayland(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	displaydensity.RunTest(ctx, s, pre.TestAPIConn, pre.Container, "wayland", crostini.WaylandDemoAppPath)
}
