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
		Func:         SanityDownload,
		Desc:         "Tests basic Crostini startup only (where crostini was downloaded first)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      10 * time.Minute,
		Pre:          crostini.StartedByDownload(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// SanityDownload runs the most basic crostini test and downloads the VM image
// from the staging bucket, i.e. it emulates the setup-flow that a user has.
func SanityDownload(ctx context.Context, s *testing.State) {
	sanity.RunTest(ctx, s, s.PreValue().(crostini.PreData).Container)
}
