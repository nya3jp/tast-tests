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
			"ereth@chromium.org",
			"arc-core@google.com",
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		// TODO(yusukes,ricardoq): ARCVM does not need the test. Remove this once we retire ARC container.
		SoftwareDeps: []string{
			"android_p",
			"chrome",
		},
		Attr:    []string{"group:mainline"},
		Fixture: "arcBooted",
	})
}

func ContainerMount(ctx context.Context, s *testing.State) {
	containermount.RunTest(ctx, s, s.FixtValue().(*arc.PreData).ARC)
}
