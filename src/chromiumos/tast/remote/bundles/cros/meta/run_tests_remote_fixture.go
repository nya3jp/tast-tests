// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"path/filepath"
	"regexp"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsRemoteFixture,
		Desc:     "Verifies that Tast can run remote fixtures",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
	})
}

func RunTestsRemoteFixture(ctx context.Context, s *testing.State) {
	resultsDir := filepath.Join(s.OutDir(), "subtest_results")
	flags := []string{
		"-build=false",
		"-resultsdir=" + resultsDir,
	}
	stdout, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"meta.LocalRemoteFixture"})
	if err != nil {
		s.Fatalf("Failed to run tast: %v", err)
	}

	re := regexp.MustCompile(`(?s)metaRemote - SetUp.*Hello LocalRemoteFixture.*metaRemote - TearDown`)
	if !re.Match(stdout) {
		s.Error("Log (stdout) didn't match with %s; see stdout.txt", re)
	}
}
