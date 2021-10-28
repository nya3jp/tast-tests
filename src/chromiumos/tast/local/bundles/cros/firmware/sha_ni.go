// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShaNI,
		Desc: "Run SHA-NI extension test on x86 platforms",
		Contacts: []string{
			"khwon@chromium.org",           // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.X86()),
	})
}

func ShaNI(ctx context.Context, s *testing.State) {
	testBin := "/usr/local/share/vboot/tests/vb2_sha256_x86_tests"
	cmd := testexec.CommandContext(ctx, testBin)
	out, err := cmd.Output(testexec.DumpLogOnError)
	outs := string(out)
	if err != nil || strings.Contains(outs, "FAILED") {
		path := filepath.Join(s.OutDir(), "sha_ni.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save SHA-NI test output: ", err)
		}
		s.Fatalf("SHA-NI test failed (saved output to %s)", filepath.Base(path))
	}
}
