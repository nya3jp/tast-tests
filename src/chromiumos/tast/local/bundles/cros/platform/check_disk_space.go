// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"math"

	"chromiumos/tast/local/bundles/cros/platform/fsinfo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckDiskSpace,
		Desc:     "Checks that sufficient space is available in the root filesystem",
		Contacts: []string{"norvez@chromium.org", "sarthakkukreti@chromium.org", "chromeos-storage@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

func CheckDiskSpace(ctx context.Context, s *testing.State) {
	info, err := fsinfo.Get(ctx, "/")
	if err != nil {
		s.Fatal("Failed to get information about root filesystem: ", err)
	}

	toMiB := func(b int64) int64 { return int64(math.Round((float64(b) / (1024 * 1024)))) }
	s.Logf("Root filesystem of type %v is using %v bytes (%v MiB) with %v bytes available (%v MiB)",
		info.Type, info.Used, toMiB(info.Used), info.Avail, toMiB(info.Avail))

	if info.Type == "squashfs" {
		s.Log("Not checking available space since it's always 0 with squashfs")
		return
	}
	// Require the minimum of 32 MiB and 4% of total.
	var req int64 = 32 * 1024 * 1024
	if b := int64(0.04 * float64(info.Used+info.Avail)); b < req {
		req = b
	}
	if info.Avail < req {
		s.Errorf("Root filesystem has %d bytes (%v MiB) available; %d bytes (%v MiB) are required",
			info.Avail, toMiB(info.Avail), req, toMiB(req))
	}
}
