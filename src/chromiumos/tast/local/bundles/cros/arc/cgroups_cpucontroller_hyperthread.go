// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cgroups"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CgroupsCpucontrollerHyperthread,
		Desc: "Verifies hyperthreading option enables within ARC",
		Contacts: []string{
			"arc-core@google.com",
			"ereth@google.com",
		},
		SoftwareDeps: []string{
			"android",
			"chrome",
		},
		Attr:    []string{"informational"},
		Timeout: 4 * time.Minute,
	})
}

func CgroupsCpucontrollerHyperthread(ctx context.Context, s *testing.State) {
	// Login with Hyperthreading enabled (if supported by hardware)
	_, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--scheduler-configuration=performance"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}

	cgroups.TestCPUSet(ctx, s, a)
}
