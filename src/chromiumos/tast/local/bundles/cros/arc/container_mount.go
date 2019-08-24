// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/containermount"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ContainerMount,
		Desc: "Verifies mount points' shared flags for ARC",
		Contacts: []string{
			"hidehiko@chromium.org",
			"arc-storage@google.com",
		},
		SoftwareDeps: []string{
			"android",
			"chrome",
		},
		Attr: []string{"informational"},
		Pre:  arc.Booted(),
	})
}

func ContainerMount(ctx context.Context, s *testing.State) {
	containermount.RunTest(ctx, s, s.PreValue().(arc.PreData).ARC)
}
