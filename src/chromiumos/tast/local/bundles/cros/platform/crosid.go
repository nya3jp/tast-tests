// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosID,
		Desc: "Tests that unibuild devices are able to probe identity via crosid",
		Contacts: []string{
			"jrosenth@chromium.org", // Test author
		},
		SoftwareDeps: []string{"cros_config"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrosID(ctx context.Context, s *testing.State) {
	out, err := testexec.CommandContext(ctx, "crosid", "-v").CombinedOutput()
	if err != nil {
		s.Fatalf("crosid exited with status %s: %s", testexec.ExitCode(err), string(out))
	}
}
