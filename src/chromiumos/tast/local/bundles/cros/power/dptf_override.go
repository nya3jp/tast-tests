// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DptfOverride,
		Desc: "Check that dptf loads correct thermal profile",
		Contacts: []string{
			"puthik@chromium.org",                // test author
			"chromeos-platform-power@google.com", // CrOS platform power developers
		},
		Attr: []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.Platform(
			"atlas", "coral", "drallion", "fizz", "hatch", "nami", "octopus", "poppy")),
	})
}

func DptfOverride(ctx context.Context, s *testing.State) {
	expectedProfile := ""
	actualProfile := ""
	var cmds []string

	// Use dptf_override .sh if exists, otherwise use cros_config
	_, err := os.Stat("/etc/dptf/dptf_override.sh")
	if err == nil {
		cmds = []string{"sh", "-c", ". /etc/dptf/dptf_override.sh; dptf_get_override"}
	} else if os.IsNotExist(err) {
		cmds = []string{"cros_config", "/thermal", "dptf-dv"}
	} else {
		s.Fatal("Unexpected os.Stat error: ", err)
	}

	// Run the command to get expected DPTF profile
	out, err := exec.Command(cmds[0], cmds[1:]...).CombinedOutput()
	if err != nil {
		// If there is no DPTF profile, the command shouldn't return 0
		s.Log("Expected DPTF profile: ''")
	} else if len(out) > 0 {
		expectedProfile = "/etc/dptf/" + strings.TrimSpace(string(out))
		s.Logf("Found DPTF profile via %s: %q", strings.Join(cmds, " "), expectedProfile)
	} else {
		s.Fatalf("Can't DPTF profile via %s", strings.Join(cmds, " "))
	}

	// Use pgrep to get actual DPTF profile
	out, err = exec.Command("pgrep", "-a", "esif_ufd").CombinedOutput()
	if err != nil {
		s.Fatal("Search for DPTF process failed: ", err)
	}
	if len(out) == 0 {
		s.Fatal("Can't find DPTF process")
	}
	outSlice := strings.Fields(string(out))
	last := outSlice[len(outSlice)-1]
	if strings.HasPrefix(last, "/etc/dptf/") {
		actualProfile = last
	}

	s.Logf("Actual DPTF profile loaded: %q", actualProfile)
	if expectedProfile != actualProfile {
		s.Fatalf("DPTF profile not matched. Expected: %q Actual: %q",
			expectedProfile, actualProfile)
	}
}
