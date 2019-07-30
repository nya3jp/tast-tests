// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/copypaste"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CopyPasteWaylandToWaylandArtifact,
		Desc:         "Test wayland copy paste functionality (where crostini was shipped with the build)",
		Contacts:     []string{"sidereal@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyPasteWaylandToWaylandArtifact(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	copypaste.RunTest(ctx, s, pre.TestAPIConn, pre.Container,
		copypaste.WaylandCopyConfig,
		copypaste.WaylandPasteConfig)
}
