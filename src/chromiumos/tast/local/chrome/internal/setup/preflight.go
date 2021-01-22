// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// PreflightCheck runs several checks that are nice to be performed before
// restarting Chrome for testing.
func PreflightCheck(ctx context.Context, cfg *config.Config) error {
	if err := checkSoftwareDeps(ctx); err != nil {
		return err
	}

	if err := checkStateful(); err != nil {
		return err
	}

	// Perform an early high-level check of cryptohomed to avoid
	// less-descriptive errors later if it's broken.
	if cfg.LoginMode != config.NoLogin {
		if err := cryptohome.CheckService(ctx); err != nil {
			// Log problems in cryptohomed's dependencies.
			for _, e := range cryptohome.CheckDeps(ctx) {
				testing.ContextLog(ctx, "Potential cryptohome issue: ", e)
			}
			return errors.Wrap(err, "failed to check cryptohome service")
		}
	}

	return nil
}

// checkSoftwareDeps ensures the current test declares necessary software dependencies.
func checkSoftwareDeps(ctx context.Context) error {
	deps, ok := testing.ContextSoftwareDeps(ctx)
	if !ok {
		// Test info can be unavailable in unit tests.
		return nil
	}

	const needed = "chrome"
	for _, dep := range deps {
		if dep == needed {
			return nil
		}
	}
	return errors.Errorf("test must declare %q software dependency", needed)
}

// checkStateful ensures that the stateful partition is writable.
// This check help debugging in somewhat popular case where disk is physically broken.
// TODO(crbug.com/1047105): Consider moving this check to pre-test hooks if it turns out to be useful.
func checkStateful() error {
	for _, dir := range []string{
		"/mnt/stateful_partition",
		"/mnt/stateful_partition/encrypted",
	} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // some dirs may not exist (e.g. on moblab)
		} else if err != nil {
			return errors.Wrapf(err, "failed to stat %s", dir)
		}
		fp := filepath.Join(dir, ".tast.check-disk")
		if err := ioutil.WriteFile(fp, nil, 0600); err != nil {
			return errors.Wrapf(err, "%s is not writable", dir)
		}
		if err := os.Remove(fp); err != nil {
			return errors.Wrapf(err, "%s is not writable", dir)
		}
	}
	return nil
}
