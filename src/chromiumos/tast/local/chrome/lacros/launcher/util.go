// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// prepareLacrosChromeBinary ensures that lacros-chrome binary is available on
// disk and ready to launch. Does not launch the binary.
// This will extract lacros-chrome to where the lacrosRootPath constant points to.
func prepareLacrosChromeBinary(ctx context.Context, s *testing.FixtState) error {
	testing.ContextLog(ctx, "Preparing the environment to run Lacros")
	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	if err := os.Chown(lacrosTestPath, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return errors.Wrap(err, "failed to chown test artifacts directory")
	}

	testing.ContextLog(ctx, "Extracting lacros binary")
	tarCmd := testexec.CommandContext(ctx, "sudo", "-E", "-u", "chronos",
		"tar", "-xvf", s.DataPath(DataArtifact), "-C", lacrosTestPath)

	if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to untar test artifacts")
	}

	if err := os.Chmod(lacrosRootPath, 0777); err != nil {
		return errors.Wrap(err, "failed to change permissions of the binary root dir path")
	}

	return nil
}

// ServePolicyAndRefresh updates the policy of FakeDMS and refreshes the policies in Lacros Chrome.
func ServePolicyAndRefresh(ctx context.Context, fdms *fakedms.FakeDMS, lacros *LacrosChrome, ps []policy.Policy) error {
	// Make sure FakeDMS is still running.
	if err := fdms.Ping(ctx); err != nil {
		return errors.Wrap(err, "failed to ping FakeDMS")
	}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(ps)
	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policies to FakeDMS")
	}

	tconn, err := lacros.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	return policyutil.Refresh(ctx, tconn)
}
