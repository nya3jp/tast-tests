// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

// FreeSpace returns the number of free bytes available at a specific path
func FreeSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, errors.Wrapf(err, "failed to get disk stats for %s", path)
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
