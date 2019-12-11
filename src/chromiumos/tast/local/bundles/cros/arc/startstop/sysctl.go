// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	expectNonARCFileListKiB = 100000
	expectARCFileListKiB    = 400000
)

// TestSysctl runs inside arc.StartStop, which verifies sysctl settings for
// ARC container.
type TestSysctl struct{}

// Name returns the name of this subtest.
func (*TestSysctl) Name() string { return "Sysctl" }

// PreStart implements Subtest.PreStart().
func (t *TestSysctl) PreStart(ctx context.Context, s *testing.State) {
	t.verify(ctx, s, expectNonARCFileListKiB)
}

// PostStart implements Subtest.PreStart().
func (t *TestSysctl) PostStart(ctx context.Context, s *testing.State) {
	t.verify(ctx, s, expectARCFileListKiB)
}

// PostStop implements Subtest.PostStop().
func (t *TestSysctl) PostStop(ctx context.Context, s *testing.State) {
	t.verify(ctx, s, expectNonARCFileListKiB)
}

func (t *TestSysctl) verify(ctx context.Context, s *testing.State, expect int) {
	out, err := testexec.CommandContext(ctx, "sysctl", "-n", "vm.min_filelist_kbytes").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get vm.min_filelist_kbytes: ", err)
	}
	if val, err := strconv.Atoi(strings.TrimSpace(string(out))); err != nil {
		s.Error("Failed to parse sysctl output: ", err)
	} else if val != expect {
		s.Errorf("Unexpected vm.min_filelist_kbytes: got %d; want %d", val, expect)
	}
}
