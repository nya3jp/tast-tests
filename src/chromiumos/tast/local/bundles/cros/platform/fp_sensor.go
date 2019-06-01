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
		Func: FpSensor,
		Desc: "Checks that ectool commands for fpsensor behave as expected",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
		},
		Attr: []string{"informational"},
	})
}

func FpSensor(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "ectool", "--name=cros_fp", "fpencstatus")
	s.Logf("Running command: %q", shutil.EscapeSlice(cmd.Args))
	if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
	} else if !strings.Contains(string(out), "FPTPM seed set:1") {
		s.Error("FPTPM seed is not set")
	} else {
		s.Log("Succees")
	}
}
