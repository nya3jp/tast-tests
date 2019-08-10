// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cgroups"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CgroupsCpucontroller,
		Desc: "Verifies cpu sets for ARC",
		Contacts: []string{
			"arc-core@google.com",
			"ereth@google.com",
		},
		SoftwareDeps: []string{
			"android",
			"chrome",
		},
		Attr:    []string{"informational"},
		Pre:     arc.Booted(),
		Timeout: 4 * time.Minute,
	})
}

func CgroupsCpucontroller(ctx context.Context, s *testing.State) {
	cgroups.TestCPUSet(ctx, s, s.PreValue().(arc.PreData).ARC)
}
