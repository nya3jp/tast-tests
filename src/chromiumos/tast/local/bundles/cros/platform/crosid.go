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
			"chromeos-config@google.com",
		},
		SoftwareDeps: []string{"unibuild"},
		Attr:         []string{"group:mainline"},
	})
}

func CrosID(ctx context.Context, s *testing.State) {
	out, err := testexec.CommandContext(ctx, "crosid", "-v").CombinedOutput()
	status, ok := testexec.ExitCode(err)
	if !ok {
		s.Fatalf("crosid exited with status %d: %s", status, string(out))
	}
}
