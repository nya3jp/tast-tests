// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"context"
	"io/ioutil"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// DropCaches will write dirty pages to disks with 'sync' so that they are not
// available for freeing and thus helps drop_caches to free more memory from
// file system and IO operations. 'drop_caches' will clear clean page caches,
// dentries (directory caches), and inodes (file caches).
// 'value' options:
// - 1: Clears page caches
// - 2: Clears dentries and inodes
// - 3: Clears page caches, dentries and inodes
func DropCaches(ctx context.Context, value int) error {
	if value < 1 || value > 3 {
		return errors.Errorf("invalid drop_caches value: %d", value)
	}

	v := strconv.Itoa(value)
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to flush buffers")
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte(v), 0200); err != nil {
		return errors.Wrap(err, "failed to clear caches")
	}
	return nil
}
