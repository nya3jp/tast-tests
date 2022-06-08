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
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type cpuSetConfig struct {
	// Extra Chrome command line options
	chromeExtraArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CPUSet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies cpuset",
		Contacts: []string{
			"yusukes@chromium.org",
			"arc-core@google.com",
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		// "no_qemu" is added for excluding betty from the target board list. b/196907826
		SoftwareDeps: []string{
			"chrome",
			"no_qemu",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               cpuSetConfig{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: cpuSetConfig{
				// Make sure the DUT uses per-VM core scheduling rather than per-vCPU one. This will prevent the test
				// from failing even when ArcEnablePerVmCoreScheduling's default in components/arc/arc_features.cc is
				// changed. When ArcEnablePerVmCoreScheduling's default is changed, the flag below should eventually
				// be changed too.
				chromeExtraArgs: []string{"--enable-features=ArcEnablePerVmCoreScheduling",
					// Similarly, make sure the DUT won't set up RT vCPU for all machines at the moment.
					// This will prevent the test from failing even when [ArcRtVcpuDualCore|ArcRtVcpuQuadCore]'s default
					// in components/arc/arc_features.cc is changed. When [ArcRtVcpuDualCore|ArcRtVcpuQuadCore]'s default
					// is changed, the flags below should eventually be changed too.
					"--disable-features=ArcRtVcpuDualCore,ArcRtVcpuQuadCore"},
			},
		}},
		Timeout: 7 * time.Minute,
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

	cfg := s.Param().(cpuSetConfig)

	// Shorten the total context to make room for cleanup jobs.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU(),
		chrome.ExtraArgs(cfg.chromeExtraArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(cleanupCtx); err != nil {
			s.Fatal("Failed to close Chrome: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	// Verify that /dev/cpuset is properly set up.
	types := []string{"foreground", "background", "system-background", "top-app", "restricted"}
	numOtherCores := 0

	numExpectedGuestCpus := runtime.NumCPU()
	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to determine guest type: ", err)
	}
	if isVMEnabled {
		// Don't run the test on rammus-arc-r. Although ARCVM doesn't officially support host kernel
		// 4.4, rammus-arc-r which exists for development purposes is the exception. Unfortunately,
		// 4.4 kernel doesn't support core scheduling, and its crosvm is started with an unusual number
		// of vCPUs. For now, we want to skip the test on rammus-arc-r. Once its host kernel is updated
		// to a recent version in M96, the test will start passing on rammus-arc-r.
		// TODO(yusukes): Remove this workaround once rammus-arc-r's host kernel is updated.
		if ver, _, err := sysutil.KernelVersionAndArch(); err != nil {
			s.Fatal("Failed to get kernel version: ", err)
		} else if ver.Is(4, 4) {
			return
		}
	}

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
			if len(cpusInUse) != numExpectedGuestCpus {
				s.Errorf("Unexpected CPU setting %q for %s: got %d CPUs, want %d CPUs", val, path,
					len(cpusInUse), numExpectedGuestCpus)
			}
		} else {
			// Other processes should not.
			if len(cpusInUse) == numExpectedGuestCpus {
				s.Errorf("Unexpected CPU setting %q for %s: should not be %d CPUs", val, path,
					numExpectedGuestCpus)
			}
			numOtherCores += len(cpusInUse)
		}
	}

	heterogeneousCores, err := isHeterogeneousCores(ctx)
	if err != nil {
		s.Fatal("Failed to determine core type: ", err)
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
