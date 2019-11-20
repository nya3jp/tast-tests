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
			"quasisec@chromium.org",        // Test Author
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"flashrom", "flashrom_tester"},
	})
}

// FlashromAVL runs the flashrom_tester utility and re-qualifies each SPI/chipset combo.
func FlashromAVL(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "flashrom_tester", "/usr/sbin/flashrom", "host", "Get device name", "Coreboot ELOG sanity")
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else {
		path := filepath.Join(s.OutDir(), "flashrom_tester.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save flashrom_tester output: ", err)
		}
	}
}
