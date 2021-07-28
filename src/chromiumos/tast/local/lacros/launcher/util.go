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
func prepareLacrosChromeBinary(ctx context.Context, s *testing.FixtState, path string) error {
	testing.ContextLog(ctx, "Preparing the environment to run Lacros")
	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	artifactPath := s.DataPath(DataArtifact)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		testing.ContextLog(ctx, "Extracting lacros binary")
		tarCmd := testexec.CommandContext(ctx, "tar", "-xvf", artifactPath, "-C", lacrosTestPath)
		if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to untar test artifacts")
		}

		if err := os.Chmod(path, 0777); err != nil {
			return errors.Wrap(err, "failed to change permissions of the binary root dir path")
		}
	} else {
		return err
	}

	return nil
}
