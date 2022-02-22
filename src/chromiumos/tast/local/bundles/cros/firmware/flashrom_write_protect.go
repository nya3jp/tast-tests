// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FlashromWriteProtect,
		Desc:         "Checks that flashrom supports writeprotect commands on the device's flash IC",
		Contacts:     []string{"nartemiev@google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"flashrom"},
	})
}

func FlashromWriteProtect(ctx context.Context, s *testing.State) {
	// WP support is all-or-nothing, just run a simple status command
	// to check the flash IC is supported
	var cmd = testexec.CommandContext(ctx, "flashrom", "--wp-status")

	var output, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %s", shutil.EscapeSlice(cmd.Args), err)
	} else {
		s.Logf("%q output: %s", shutil.EscapeSlice(cmd.Args), output)
	}
}
