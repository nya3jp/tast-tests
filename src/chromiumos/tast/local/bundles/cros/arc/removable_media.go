// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemovableMedia,
		Desc: "Verifies ARC removable media integration is working",
		Contacts: []string{
			"hashimoto@chromium.org", // original author
			"hidehiko@chromium.org",  // Tast port author
			"arc-storage@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"capybara.jpg"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
			Pre:               arc.VMBooted(),
		}},
	})
}

func RemovableMedia(ctx context.Context, s *testing.State) {
	removablemedia.RunTest(ctx, s, s.PreValue().(arc.PreData).ARC, "capybara.jpg")
}
