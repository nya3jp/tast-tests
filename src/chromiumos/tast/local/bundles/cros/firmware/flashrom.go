// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Flashrom,
		Desc: "Checks that flashrom can find a SPI ROM",
		Contacts: []string{
			"kmshelton@chromium.org",       // Test Author
			"quasisec@chromium.org",        // CrOS Flashrom Maintainer
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline", "group:labqual"},
		SoftwareDeps: []string{"flashrom"},
	})
}

// Flashrom runs the flashrom utility and confirms that flashrom was able to
// communicate with a SPI ROM.
func Flashrom(ctx context.Context, s *testing.State) {
	// This test intentionally avoids SPI ROM read and write operations, so as not
	// to stress devices-under-test.
	cmd := testexec.CommandContext(ctx, "flashrom", "--verbose")
	re := regexp.MustCompile(`Found .* flash chip`)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if outs := string(out); !re.MatchString(outs) {
		path := filepath.Join(s.OutDir(), "flashrom.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save flashrom output: ", err)
		}
		s.Fatalf("Failed to confirm flashrom could find a flash chip.  "+
			"Output of %q did not contain %q (saved output to %s).",
			shutil.EscapeSlice(cmd.Args), re, filepath.Base(path))
	}
}
