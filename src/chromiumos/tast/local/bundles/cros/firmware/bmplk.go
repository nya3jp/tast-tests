// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "This test checks whether the BIOS was built with a correctly configured
    		bmpblk to ensure crisp firmware screen images and text. The bmpblk for every
    		device needs to be explicitly configured for the device's screen resolution
    		to ensure optimal quality. Relies on flashrom and cbfstool to inspect BIOS.",
		Contacts: []string{
			"jwerner@chromium.org",      // Test author
			"kmshelton@chromium.org",      // Test porter (from TAuto)
			"chromeos-faft@chromium.org", // Backup mailing list
		},
		// TODO: Move to firmware_unstable, then firmware_bios
		Attr: []string{"group:firmware", "firmware_experimental"},
	})
}

func bmpblk(ctx context.Context, s *testing.State) {
	// TODO(kmshelton): Dump the whole AP firmware to a file.
	const printCBFSCmd uint32 = 'cbfstool /tmp/test_file.bin print'
	const layoutCBFSCmd = 'cbfstool /tmp/test_file.bin layout'
	re := regexp.MustCompile(`BOOT_STUB`)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if outs := string(out); !re.MatchString(outs) {
		path := filepath.Join(s.OutDir(), "cbfs.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save cbfs output: ", err)
		}
		s.Fatalf("",
			shutil.EscapeSlice(cmd.Args), re, filepath.Base(path))
	}
}
}
