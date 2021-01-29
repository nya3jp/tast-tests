// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"context"
	"os"
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

	artifactPath := s.DataPath(DataArtifact)
	tarCmd := testexec.CommandContext(ctx, "tar", "-xvf", artifactPath, "-C", lacrosTestPath)
	if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to untar test artifacts")
	}
	if err := os.Chmod(binaryPath, 0777); err != nil {
		return errors.Wrap(err, "failed to change permissions of binary dir path")
	}

	return nil
}
