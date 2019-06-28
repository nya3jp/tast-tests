// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/vm/crostini/sanity"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SanityArtifact,
		Desc:         "Tests basic Crostini startup only (where crostini was shipped with the build)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{vm.CrostiniImageArtifact},
		Pre:          vm.CrostiniStartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// SanityArtifact runs the sanity crostini test and uses a pre-built image
// artifact to initialize the VM.
func SanityArtifact(ctx context.Context, s *testing.State) {
	sanity.Sanity(ctx, s)
}
