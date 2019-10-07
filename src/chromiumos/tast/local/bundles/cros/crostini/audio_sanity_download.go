// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/audiosanity"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioSanityDownload,
		Desc:         "Runs a sanity test on the container's audio (through alsa) when downloading the crostini image",
		Contacts:     []string{"paulhsia@chromium.org", "cros-containers-dev@google.com", "chromeos-audio-bugs@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Pre:          crostini.StartedByDownload(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func AudioSanityDownload(ctx context.Context, s *testing.State) {
	audiosanity.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}
