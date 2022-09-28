// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/firmware"
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

// sysfsRootDeviceSize fetches the root device size from /sys/block/<dev>/size.
func sysfsRootDeviceSize(ctx context.Context) (int64, error) {
	// Check actual root device size.
	rootdev, err := firmware.RootDevice(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch root device")
	}

	fp := fmt.Sprintf("/sys/block/%s/size", filepath.Base(rootdev))
	content, err := ioutil.ReadFile(fp)
	if err != nil {
		return 0, errors.Wrapf(err, "reading filepath %s", fp)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse root device size as int64")
	}

	// Size is in sectors; return in bytes.
	return size * 512, nil
}

// statFreeDiskSpace gets the free space on the filesystem using statfs().
func statFreeDiskSpace(ctx context.Context, path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, errors.Wrapf(err, "failed to get disk stats for %s", path)
	}

	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// statTotalDiskSpace gets the total space on the filesystem using statfs().
func statTotalDiskSpace(ctx context.Context, path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, errors.Wrapf(err, "failed to get disk stats for %s", path)
	}

	return int64(stat.Blocks) * int64(stat.Bsize), nil
}

func Spaced(ctx context.Context, s *testing.State) {
	const (
		// Path to check disk space queries on.
		statefulMount = "/mnt/stateful_partition/"
		// Disk space margin to consider when comparing against expected values.
		spaceMarginBytes = 100 * 1024 * 1024
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

	expectedRootDeviceSize, err := sysfsRootDeviceSize(ctx)
	if err != nil {
		s.Fatal("Failed to get actual root device size: ", err)
	}

	if rootDeviceSize != expectedRootDeviceSize {
		s.Fatalf("Invalid root device size: got %d, want: 0 < size < %d", rootDeviceSize, expectedRootDeviceSize)
	}

	freeDiskSpace, err := spaced.FreeDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query free disk space: ", err)
	}

	expectedFreeDiskSpace, err := statFreeDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to get expected free disk space: ", err)
	}

	if freeDiskSpace <= 0 || freeDiskSpace > expectedFreeDiskSpace+spaceMarginBytes {
		s.Fatalf("Invalid free disk space;  got %d, want: 0 < size < %d", freeDiskSpace, expectedFreeDiskSpace+spaceMarginBytes)
	}

	totalDiskSpace, err := spaced.TotalDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to query total disk space: ", err)
	}

	expectedTotalDiskSpace, err := statTotalDiskSpace(ctx, statefulMount)
	if err != nil {
		s.Fatal("Failed to get expected total disk space: ", err)
	}

	if totalDiskSpace <= 0 || totalDiskSpace > expectedTotalDiskSpace+spaceMarginBytes {
		s.Fatalf("Invalid total disk space;  got %d, want: 0 < size < %d", totalDiskSpace, expectedTotalDiskSpace+spaceMarginBytes)
	}
}
