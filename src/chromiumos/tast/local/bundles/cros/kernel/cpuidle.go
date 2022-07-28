// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Cpuidle,
		Desc: "Ensures the system is running the expected cpuidle governor",
		Contacts: []string{
			"briannorris@chromium.org",
			"swboyd@chromium.org",
			"baseos-perf@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cpuidle_teo"},
	})
}

// Cpuidle checks the active cpuidle governor and matches it against the expectations for the given
// platform/architecture.
func Cpuidle(ctx context.Context, s *testing.State) {
	ver, arch, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version and arch: ", err)
	}

	// TEO governor is available on CrOS kernels >= 4.19. If we fail this, we have the
	// 'cpuidle_teo' dependency wrong.
	if !ver.IsOrLater(4, 19) {
		s.Fatal("Unexpected kernel version: ", ver)
	}

	// Pre-kernel-v5.8, current_governor and current_governor_ro were
	// mutually exclusive.
	data, err := os.ReadFile("/sys/devices/system/cpu/cpuidle/current_governor_ro")
	if err != nil {
		if !os.IsNotExist(err) {
			s.Fatal("Failed to read cpuidle current_governor_ro: ", err)
		}
		data, err = os.ReadFile("/sys/devices/system/cpu/cpuidle/current_governor")
		if err != nil {
			s.Fatal("Failed to read cpuidle current_governor: ", err)
		}
	}
	governor := strings.TrimSpace(string(data))

	// See b/172228121, b/238655078, b/239626992, b/196200718. Currently, Arm systems tend to
	// have better performance and equivalent battery life on TEO. x86 is sticking with Menu.
	var expectedGovernor string
	switch arch {
	case "aarch64":
		expectedGovernor = "teo"
	case "x86_64":
		expectedGovernor = "menu"
	default:
		s.Fatal("Unexpected architecture: ", arch)
	}

	if governor != expectedGovernor {
		s.Fatalf("Expected cpuidle governor: got %q, want %q", governor, expectedGovernor)
	}
}
