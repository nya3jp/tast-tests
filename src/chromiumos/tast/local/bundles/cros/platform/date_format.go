// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DateFormat,
		Desc: "Checks that the date command prints dates as expected",
		Contacts: []string{
			"me@chromium.org",         // Test author
			"tast-users@chromium.org", // Backup mailing list
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func DateFormat(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		date string // value to pass via --date flag
		spec string // spec to pass in "+"-prefixed arg
		exp  string // expected UTC output (minus trailing newline)
	}{
		{"2004-02-29 16:21:42 +0100", "%Y-%m-%d %H:%M:%S", "2004-02-29 15:21:42"},
		{"Sun, 29 Feb 2004 16:21:42 -0800", "%Y-%m-%d %H:%M:%S", "2004-03-01 00:21:42"},
	} {
		// Test body will go here.
		cmd := testexec.CommandContext(ctx, "date", "--utc", "--date="+tc.date, "+"+tc.spec)
		if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
		} else if outs := strings.TrimRight(string(out), "\n"); outs != tc.exp {
			s.Errorf("%q printed %q; want %q", shutil.EscapeSlice(cmd.Args), outs, tc.exp)
		}
	}

}
