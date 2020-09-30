// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"runtime"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CPUSet,
		Desc: "Verifies mount points' shared flags for ARC",
		Contacts: []string{
			"ereth@chromium.org",
			"arc-core@google.com",
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{
			"android_vm",
			"chrome",
		},
		Attr: []string{"group:mainline", "informational"},
		Pre:  arc.Booted(),
	})
}

// Intel does not have any 'CPU part' line. ARM does, and when it's
// big.LITTLE, it has two different CPU parts (e.g. 0xd03 and 0xd09).
var cpuPartRE = regexp.MustCompile(`^CPU part\s+:\s+(0x[0-9a-f]+)$`)

// isHeterogeneousCores returns true if cores are heterogeneous ones
// such as ARM's big.LITTLE.
func isHeterogeneousCores(ctx context.Context) (bool, error) {
	if arch := runtime.GOARCH; arch != "arm" && arch != "arm64" {
		return false, nil
	}

	out, err := ioutil.ReadFile("/proc/cpuinfo")

	if err != nil {
		return false, errors.Wrap(err, "failed to read cpuinfo")
	}
	lines := strings.Split(string(out), "\n")

	var lastCPUPart string
	for _, line := range lines {
		matches := cpuPartRE.FindAllStringSubmatch(strings.TrimSpace(line), -1)
		if matches == nil {
			continue
		}
		cpuPart := matches[0][1]
		if lastCPUPart == "" {
			lastCPUPart = cpuPart
			continue
		}
		if lastCPUPart != cpuPart {
			testing.ContextLog(ctx, "Detected heterogeneous cores")
			return true, nil
		}
	}
	if lastCPUPart == "" {
		return false, errors.New("no CPU part information found")
	}
	return false, nil
}

func testCPUSet(ctx context.Context, s *testing.State, a *arc.ARC) {
	s.Log("Running testCPUSet")

	// Verify that /dev/cpuset is properly set up.
	types := []string{"foreground", "background", "system-background", "top-app", "restricted"}
	numOtherCores := 0

	for _, t := range types {
		// cgroup pseudo file cannot be "adb pull"ed. Additionally, it is not
		// accessible via adb shell user in P. Access by procfs instead.
		initPID := 1
		path := fmt.Sprintf("/proc/%d/root/dev/cpuset/%s/effective_cpus", initPID, t)

		out, err := testexec.CommandContext(ctx, "android-sh", "-c", fmt.Sprintf("cat %s", path)).Output(testexec.DumpLogOnError)

		if err != nil {
			s.Errorf("Failed to read %s: %v", path, err)
			continue
		}
		val := strings.TrimSpace(string(out))
		cpusInUse, err := cpuset.Parse(ctx, val)
		if err != nil {
			s.Errorf("Failed to parse %s: %v", path, err)
			continue
		}

		if t == "foreground" || t == "top-app" {
			// Even after full boot, these processes should be able
			// to use all CPU cores.
			if len(cpusInUse) != runtime.NumCPU() {
				s.Errorf("Unexpected CPU setting %q for %s: got %d CPUs, want %d CPUs", val, path,
					len(cpusInUse), runtime.NumCPU())
			}
		} else {
			// Other processes should not.
			if len(cpusInUse) == runtime.NumCPU() {
				s.Errorf("Unexpected CPU setting %q for %s: should not be %d CPUs", val, path,
					runtime.NumCPU())
			}
			numOtherCores += len(cpusInUse)
		}
	}

	heterogeneousCores, err := isHeterogeneousCores(ctx)
	if err != nil {
		s.Error("Failed to determine core type: ", err)
		return
	}
	if heterogeneousCores {
		// The cpuset settings done in init.{cheets,bertha}.rc work fine
		// for homogeneous systems, but heterogeneous systems require
		// their own init.cpusets.rc to properly set up non-foreground
		// cpusets. Verify that init.cpusets.rc exists on heterogeneous
		// systems.
		s.Log("Running testCPUSet (for boards with heterogeneous cores)")
		if numOtherCores == 3 {
			s.Error("Unexpected CPU setting; found heterogeneous cores but lacks proper init.cpusets.rc file")
		}
	}
}

func CPUSet(ctx context.Context, s *testing.State) {
	testCPUSet(ctx, s, s.PreValue().(arc.PreData).ARC)
}
