// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/sanity"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SanityInstaller,
		Desc:         "Tests basic Crostini startup only (where crostini was installed via the installer)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByInstaller(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func SanityInstaller(ctx context.Context, s *testing.State) {
	sanity.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}
