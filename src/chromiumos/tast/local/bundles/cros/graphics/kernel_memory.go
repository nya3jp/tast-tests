// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/kernel"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelMemory,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Examine a kernel build CONFIG list to verify related flags",
		// TODO(syedfaaiz): Add to CQ once it is green and stable.
		Attr: []string{"group:graphics", "graphics_nightly"},
		Contacts: []string{"syedfaaiz@google.com",
			"chromeos-gfx@google.com",
		},
		Fixture: "gpuWatchDog",
		Timeout: 2 * time.Minute,
	})
}
func KernelMemory(ctx context.Context, s *testing.State) {
	testing.Sleep(ctx, 10*time.Second)
	numErrors, err := kernel.GetMemErrors(ctx)
	if numErrors > 0 {
		s.Fatal("Errors occured during Kernel Memory test: ", err)
	}
}
