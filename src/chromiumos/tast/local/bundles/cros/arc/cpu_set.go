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

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CPUSet,
		Desc: "Verifies cpuset",
		Contacts: []string{
			"ereth@chromium.org",
			"arc-core@google.com",
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		SoftwareDeps: []string{
			"chrome",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// getInitPIDs returns all PIDs corresponding to ARC init processes.
// TODO (hidehiko@): merge this and InitPID()
func getInitPIDs() ([]int, error) {
	ver, err := arc.SDKVersion()
	if err != nil {
		return nil, err
	}

	// The path to the ARC init executable.
	var initExecPath = "/init"
	if ver >= arc.SDKQ {
		initExecPath = "/system/bin/init"
	}

	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	var pids []int
	for _, pid := range all {
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Assume that the process exited.
			continue
		}
		if exe, err := proc.Exe(); err == nil && exe == initExecPath {
			if username, err := proc.Username(); err == nil && username == "android-root" {
				pids = append(pids, int(pid))
			}
		}
	}
	return pids, nil
}

// getRootPID returns the PID of the root ARC init process.
func getRootPID() (int, error) {
	pids, err := getInitPIDs()
	if err != nil {
		return -1, err
	}

	pm := make(map[int]struct{}, len(pids))
	for _, pid := range pids {
		pm[pid] = struct{}{}
	}
	for _, pid := range pids {
		// If we see errors, assume that the process exited.
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}
		ppid, err := proc.Ppid()
		if err != nil || ppid <= 0 {
			continue
		}
		if _, ok := pm[int(ppid)]; !ok {
			return pid, nil
		}
	}
	return -1, errors.New("root not found")
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

func readCPUSetInfo(ctx context.Context, t string) (string, []byte, error) {
	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		return "", nil, err
	}
	if isVMEnabled {
		path := fmt.Sprintf("/proc/1/root/dev/cpuset/%s/effective_cpus", t)
		out, err := testexec.CommandContext(ctx, "android-sh", "-c", fmt.Sprintf("cat %s", shutil.Escape(path))).Output(testexec.DumpLogOnError)
		return path, out, err
	}

	initPID, err := getRootPID()
	if err != nil {
		return "", nil, err
	}
	path := fmt.Sprintf("/proc/%d/root/dev/cpuset/%s/effective_cpus", initPID, t)
	// cgroup pseudo file cannot be "adb pull"ed. Additionally, it is not
	// accessible via adb shell user in P. Access by procfs instead.
	out, err := ioutil.ReadFile(path)
	return path, out, err
}

func CPUSet(ctx context.Context, s *testing.State) {
	s.Log("Running testCPUSet")

	// Verify that /dev/cpuset is properly set up.
	types := []string{"foreground", "background", "system-background", "top-app", "restricted"}
	numOtherCores := 0

	for _, t := range types {
		// cgroup pseudo file cannot be "adb pull"ed. Additionally, it is not
		// accessible via adb shell user in P. Access by procfs instead.
		path, out, err := readCPUSetInfo(ctx, t)

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
