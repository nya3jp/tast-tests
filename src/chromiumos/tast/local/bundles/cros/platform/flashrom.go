// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
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
		Attr: []string{"informational"},
	})
}

// Flashrom runs the flashrom utility and confirms that flashrom was able to
// communicate with a SPI ROM.
func Flashrom(ctx context.Context, s *testing.State) {
	// This test intentionally avoids SPI ROM read and write operations, so as not
	// to stress devices-under-test.
	cmd := testexec.CommandContext(ctx, "flashrom", "--verbose")
	exp := "Found .* flash chip"
	re := regexp.MustCompile(exp)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if outs := string(out); !re.MatchString(outs) {
		s.Errorf("%q printed %q; want %q", shutil.EscapeSlice(cmd.Args), outs, exp)
	}
}
