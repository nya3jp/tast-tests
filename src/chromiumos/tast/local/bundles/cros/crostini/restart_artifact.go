// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/restart"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestartArtifact,
		Desc:         "Tests that we can shut down and restart crostini (where the VM image is a build artifact)",
		Contacts:     []string{"hollingum@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func RestartArtifact(ctx context.Context, s *testing.State) {
	restart.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container, 2 /*numRestarts*/)
}
