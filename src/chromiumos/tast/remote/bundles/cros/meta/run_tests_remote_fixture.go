// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/remote/bundles/cros/meta/tastrun"
	"chromiumos/tast/testing"
	// Register the fixtures to remote bundle.
	_ "chromiumos/tast/remote/bundles/cros/meta/fixture"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RunTestsRemoteFixture,
		Desc:     "Verifies that Tast can run remote fixtures",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func RunTestsRemoteFixture(ctx context.Context, s *testing.State) {
	const (
		setUpError    = "meta.metaRemote.SetUpError"
		tearDownError = "meta.metaRemote.tearDownError"
	)

	for _, tc := range []struct {
		name     string
		vars     map[string]string
		wantLogs map[string]*regexp.Regexp
	}{
		{
			name: "success",
			wantLogs: map[string]*regexp.Regexp{
				"fixtures/metaRemote/log.txt": regexp.MustCompile(`(?s)SetUp metaRemote\n.*TearDown metaRemote\n`),
				"full.txt":                    regexp.MustCompile(`(?s)SetUp metaRemote\n.*Hello test\n.*TearDown metaRemote\n`),
			},
		},
		{
			name: "setup failure",
			vars: map[string]string{
				setUpError: "Whoa",
			},
			wantLogs: map[string]*regexp.Regexp{
				"fixtures/metaRemote/log.txt":           regexp.MustCompile(`Whoa\n`),
				"tests/meta.LocalRemoteFixture/log.txt": regexp.MustCompile(`\[Fixture failure\] metaRemote: Whoa\n`),
			},
		},
		// TODO(oka): test TearDown failures after fixutre failures become
		// reported.
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			resultsDir := filepath.Join(s.OutDir(), "subtest_results")
			flags := []string{
				"-resultsdir=" + resultsDir,
			}
			for k, v := range tc.vars {
				flags = append(flags, "-var", fmt.Sprintf("%s=%s", k, v))
			}
			_, _, err := tastrun.Exec(ctx, s, "run", flags, []string{"meta.LocalRemoteFixture"})
			if err != nil {
				s.Fatal("Failed to run tast: ", err)
			}

			for k, re := range tc.wantLogs {
				if b, err := ioutil.ReadFile(filepath.Join(resultsDir, k)); err != nil {
					s.Errorf("Log %s: %v", k, err)
				} else if !re.Match(b) {
					s.Errorf("Log %s didn't match with %s", k, re)
				}
			}
		})
	}
}
