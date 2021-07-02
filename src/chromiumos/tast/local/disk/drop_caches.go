// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// DropCaches will write dirty pages to disk with 'sync' so that they are not
// available for freeing and thus helps drop_caches to free more memory from
// file system and IO operations. 'drop_caches' will clear clean page caches,
// dentries (directory caches), and inodes (file caches).
func DropCaches(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing caches, system buffer, dentries and inodes")
	if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to flush buffers")
	}
	if err := ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0200); err != nil {
		return errors.Wrap(err, "failed to clear caches")
	}
	return nil
}
