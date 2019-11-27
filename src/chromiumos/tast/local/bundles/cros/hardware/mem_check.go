// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"

	"chromiumos/tast/local/bundles/cros/hardware/memcheck"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemCheck,
		Desc: "Verifies memory usage looks correct",
		Contacts: []string{
			"puthik@chromium.org", // original test author.
			"chromeos-memory@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{"mosys"},
		Attr:         []string{"group:mainline"},
	})
}

func MemCheck(ctx context.Context, s *testing.State) {
	// This test is ported from platform_MemCheck autotest.
	// The test is also run as a part of sequence/control.memory_qual and
	// its family. At the moment, those are not yet ported, so the value
	// output feature is not implemented.
	memcheck.RunTest(ctx, s)
}
