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
		statefulMount = "/mnt/stateful_partition/"
		maxDiskSize   = 1024 * 1024 * 1024 * 1024
	)

	spaced, err := spaced.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to create spaced client: ", err)
	}

	// Check D-Bus queries.
	rootDeviceSize, err := spaced.RootDeviceSize(ctx)
	if err != nil {
		s.Fatal("Failed to query root device size: ", err)
	}

	if rootDeviceSize == 0 || rootDeviceSize > maxDiskSize {
		s.Fatalf("Invalid root device size: got %d, want: 0 < size < %d", rootDeviceSize, maxDiskSize)
	}

	freeDiskSpace, err := spaced.FreeDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query free disk space: ", err)
	}

	if freeDiskSpace == 0 || freeDiskSpace > maxDiskSize {
		s.Fatalf("Invalid free disk space;  got %d, want: 0 < size < %d", freeDiskSpace, maxDiskSize)
	}

	totalDiskSpace, err := spaced.TotalDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query total disk space: ", err)
	}

	if totalDiskSpace == 0 || totalDiskSpace > maxDiskSize {
		s.Fatalf("Invalid total disk space got %d, want: 0 < size < %d", totalDiskSpace, maxDiskSize)
	}

	if freeDiskSpace >= totalDiskSpace {
		s.Fatal("Free disk space: ", freeDiskSpace, " greater than the total disk space: ", totalDiskSpace)
	}
}
