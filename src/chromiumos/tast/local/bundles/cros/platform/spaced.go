// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/spaced"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Spaced,
		Desc:     "Checks that spaced queries work",
		Contacts: []string{"sarthakkukreti@chromium.org", "chromeos-storage@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func Spaced(ctx context.Context, s *testing.State) {
	const (
		statefulMount   = "/mnt/stateful_partition/"
		maxStatefulSize = 1024 * 1024 * 1024 * 1024
	)

	spaced, err := spaced.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to create spaced client: ", err)
	}

	// Check D-Bus queries.
	freeDiskSpace, err := spaced.GetFreeDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query free disk space: ", err)
	}

	if freeDiskSpace == 0 || freeDiskSpace > maxStatefulSize {
		s.Fatal("Invalid free disk space: ", freeDiskSpace)
	}

	totalDiskSpace, err := spaced.GetTotalDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query total disk space: ", err)
	}

	if totalDiskSpace == 0 || totalDiskSpace > maxStatefulSize {
		s.Fatal("Invalid total disk space: ", totalDiskSpace)
	}

	if freeDiskSpace >= totalDiskSpace {
		s.Fatal("Free disk space: ", freeDiskSpace, " greater than the total disk space: ", totalDiskSpace)
	}
}
