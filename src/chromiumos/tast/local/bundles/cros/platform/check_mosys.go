// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckMosys,
		Desc: "Checks the mosys command functionality",
		Contacts: []string{
			"kasaiah.bogineni@intel.com",
			"derat@chromium.org",
			"tast-users@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func CheckMosys(ctx context.Context, s *testing.State) {
	commands := [][]string{
		{"ec", "info"},
		{"platform", "name"},
		{"smbios", "info", "bios"},
		{"eeprom", "map"},
		{"platform", "vendor"},
		{"eventlog", "list"},
		{"memory", "spd", "print", "all"},
	}
	for _, mosysCmd := range commands {
		s.Logf("Verifying the command %q", shutil.EscapeSlice(mosysCmd))
		err := testexec.CommandContext(ctx, "mosys", mosysCmd...).Run(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("%q failed: %v", shutil.EscapeSlice(mosysCmd), err)
		}
	}
}
