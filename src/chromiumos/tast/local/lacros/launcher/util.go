// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// prepareLacrosChromeBinary ensures that lacros-chrome binary is available on
// disk and ready to launch. Does not launch the binary.
func prepareLacrosChromeBinary(ctx context.Context, s *testing.FixtState) error {
	mountCmd := testexec.CommandContext(ctx, "mount", "-o", "remount,exec", "/mnt/stateful_partition")
	if err := mountCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to remount stateful partition with exec privilege")
	}

	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	// TODO(crbug.com/1127165): Extract the artifact here, once we can use Data in fixtures.
	return nil
}
