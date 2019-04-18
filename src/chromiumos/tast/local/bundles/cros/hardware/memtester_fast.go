// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"

	"chromiumos/tast/local/bundles/cros/hardware/memtester"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemtesterFast,
		Desc: "Runs one iteration of memtester using 10 MiB of memory to find memory subsystem faults",
		Contacts: []string{
			"puthik@chromium.org", // Original Autotest author
			"derat@chromium.org",  // Tast port author
			"cros-partner-avl@google.com",
		},
	})
}

func MemtesterFast(ctx context.Context, s *testing.State) {
	s.Log("Testing 10 MiB")
	if err := memtester.Run(ctx, 10*1024*1024, 1); err != nil {
		s.Fatal("memtester failed: ", err)
	}
}
