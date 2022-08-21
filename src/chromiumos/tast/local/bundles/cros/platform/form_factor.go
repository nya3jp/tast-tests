// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FormFactor,
		Desc:         "Check cros_config returns a form-factor for the device",
		Contacts:     []string{"nartemiev@google.com", "chromeos-platform@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{""},
		Timeout:      3 * time.Minute,
	})
}

func FormFactor(ctx context.Context, s *testing.State) {
	var cmd = testexec.CommandContext(ctx, "cros_config", "/hardware-properties", "form-factor")

	var output, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed: %s", shutil.EscapeSlice(cmd.Args), err)
	} else if len(output) == 0 {
		s.Fatalf("%q returned empty form-factor string", shutil.EscapeSlice(cmd.Args))
	}
	s.Logf("%q returned: %s", shutil.EscapeSlice(cmd.Args), output)
}
