// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrometest

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Run executes a chrome binary test, execFileName, with args. This returns error if
// the chrome binary test fails.
func Run(ctx context.Context, execFileName string, args []string) error {
	const binaryTestDir = "/usr/local/libexec/chrome-binary-tests/"
	binaryTestPath := filepath.Join(binaryTestDir, execFileName)

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", binaryTestPath, testexec.ShellEscapeArray(args))
	cmd.Env = append(os.Environ(),
		"CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox",
	)

	testing.ContextLogf(ctx, "Executing %s %s", execFileName, testexec.ShellEscapeArray(args))
	if err := cmd.Run(); err != nil {
		defer cmd.DumpLog(ctx)
		return errors.Wrapf(err, "%s failed", binaryTestPath)
	}
	return nil
}
