// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfDmcrypt,
		Desc:         "Runs performance tests on various dmcrypt configurations",
		Contacts:     []string{"dlunev@google.com"},
		SoftwareDeps: []string{"amd64"},
		Attr:         []string{"informational"},
		Data:         []string{"perf_dmcrypt/setup_crypto.sh", "perf_dmcrypt/test_crypto.sh"},
	})
}

// PerfDmcrypt runs scripts which perf test various flag configuration of dm-crypt.
func PerfDmcrypt(ctx context.Context, s *testing.State) {
	if err := testexec.CommandContext(ctx, "bash", s.DataPath("perf_dmcrypt/setup_crypto.sh")).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to setup crypto ram device: ", err)
	}
	if err := testexec.CommandContext(ctx, "bash", s.DataPath("perf_dmcrypt/test_crypto.sh"), s.OutDir()).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to test crypto ram device: ", err)
	}
}
