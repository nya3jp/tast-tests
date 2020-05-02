// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"os/exec"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrngInit,
		Desc: "Checks if unseeded randomness is called",
		Contacts: []string{
			"hsinyi@chromium.org",
			"chromeos-kernel-test@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// CrngInit reads from syslog to check if crng is initialized when
// get random functions are called.
func CrngInit(ctx context.Context, s *testing.State) {
	out, err := exec.Command("journalctl", "-qk", "--grep", "crng_init=0").Output()
	if err != nil {
		s.Fatal("Failed to read syslog from journald: ", err)
	} else if string(out) != "" {
		s.Error("Unseeded randomness called: ", string(out))
	}
}
