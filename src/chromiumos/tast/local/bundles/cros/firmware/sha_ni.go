// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SHANI,
		Desc: "Run SHA-NI extension test on x86 platforms",
		Contacts: []string{
			"khwon@chromium.org",           // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:firmware", "firmware_ec"},
		HardwareDeps: hwdep.D(hwdep.CPUSupportsSHANI()),
	})
}

func SHANI(ctx context.Context, s *testing.State) {
	const testBin = "/usr/local/share/vboot/tests/vb2_sha256_x86_tests"
	err := testexec.CommandContext(ctx, testBin).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("vb2_sha256_x86_tests failed: ", err)
	}
}
