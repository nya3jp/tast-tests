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
		Func: GSCtool,
		Desc: "Checks that gsctool can communicate with the GSC",
		Contacts: []string{
			"kmshelton@chromium.org",       // Test Author
			"mruthven@chromium.org",        // GSC Firmware Developer
			"chromeos-gsc@google.com",      // GSC Firmware Developers
			"chromeos-firmware@google.com", // Remainder of CrOS Firmware Developers
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"gsc"},
	})
}

// GSCtool runs the gsctool utility and confirms that gsctool was able to
// communicate with the GSC (Google Security Chip) by querying its version.
func GSCtool(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "gsctool", "--any", "--fwver")
	// GSC firmware versions will be of the form <epoch>.<major>.<minor>.
	const exp = `[0-9]+\.[0-9]+\.[0-9]+`
	re := regexp.MustCompile(exp)
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if outs := string(out); !re.MatchString(outs) {
		path := filepath.Join(s.OutDir(), "gsctool.txt")
		if err := ioutil.WriteFile(path, out, 0644); err != nil {
			s.Error("Failed to save gsctool output: ", err)
		}
		s.Fatalf("Failed to find a valid GSC version: output of %q did not appear to contain a version (saved output to %s)",
			shutil.EscapeSlice(cmd.Args), filepath.Base(path))
	}
}
