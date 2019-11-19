// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Mosys,
		Desc:         "Checks the mosys command's functionality",
		SoftwareDeps: []string{"mosys"},
		Contacts: []string{
			"kasaiah.bogineni@intel.com",
			"ningappa.tirakannavar@intel.com",
		},
		Attr: []string{"group:mainline"},
		// Tests are parametrized, so that we can promote some of them
		// to critical and leave the rest as informational.
		Params: []testing.Param{
			{
				Val: [][]string{
					{"platform", "name"},
					{"eeprom", "map"},
					{"platform", "vendor"},
					{"eventlog", "list"},
				},
				ExtraAttr: []string{"informational"},
			},
			{
				Name: "ec",
				Val: [][]string{
					{"ec", "info"},
				},
				ExtraAttr: []string{"informational"},
			},
			{
				Name: "memory",
				Val: [][]string{
					{"memory", "spd", "print", "all"},
				},
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func Mosys(ctx context.Context, s *testing.State) {
	commands := s.Param().([][]string)
	for _, mosysCmd := range commands {
		s.Logf("Verifying the command %q", shutil.EscapeSlice(mosysCmd))
		cmd := testexec.CommandContext(ctx, "mosys", mosysCmd...)
		output, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("%q failed: %v", shutil.EscapeSlice(mosysCmd), err)
		} else if strings.TrimSpace(string(output)) == "" {
			s.Errorf("%q output is empty", shutil.EscapeSlice(mosysCmd))
		}
	}
}
