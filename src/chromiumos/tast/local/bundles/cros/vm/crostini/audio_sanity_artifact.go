// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/vm/crostini/audiosanity"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioSanityArtifact,
		Desc:         "Tests basic Crostini audio functions through alsa",
		Contacts:     []string{"paulhsia@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{vm.CrostiniImageArtifact},
		Pre:          vm.CrostiniStartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// AudioSanityArtifact runs a sanity test on the container's audio using a pre-built crostini image.
func AudioSanityArtifact(ctx context.Context, s *testing.State) {
	audiosanity.AudioSanity(ctx, s)
}
