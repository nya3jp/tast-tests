// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const mtd0Path = "/sys/class/mtd/mtd0"

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckMTD,
		Desc: "Verifies that mtd0 exists on ARM devices",
		Contacts: []string{
			"eizan@chromium.org",              // Test Author
			"cros-flashrom-team@chromium.org", // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com",    // CrOS Firmware Developers
		},
		Attr:         []string{"group:flashrom"},
		SoftwareDeps: []string{"flashrom", "arm"},
	})
}

func CheckMTD(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "stat", mtd0Path)
	if err := cmd.Run(); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
}
