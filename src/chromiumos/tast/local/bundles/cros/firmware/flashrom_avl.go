// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FlashromAVL,
		Desc: "Runs the Flashrom AVL qualification tooling",
		Contacts: []string{
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer (Test Author)
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:flashrom"},
		SoftwareDeps: []string{"flashrom"},
	})
}

// FlashromAVL runs the flashrom_tester utility and re-qualifies each SPI/chipset combo.
func FlashromAVL(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "flashrom_tester", "/usr/sbin/flashrom", "host", "get_device_name", "coreboot_elog_sanity")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	path := filepath.Join(s.OutDir(), "flashrom_tester.txt")
	if err := ioutil.WriteFile(path, out, 0644); err != nil {
		s.Error("Failed to save flashrom_tester output: ", err)
	}
}
