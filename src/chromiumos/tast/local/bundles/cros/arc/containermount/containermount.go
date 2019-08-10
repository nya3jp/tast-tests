// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package containermount implements the tests for ARC related mount points.
package containermount

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// mountsForMinijail returns a list of mount points of the minijail'ed process
// whose PID file is at pidFile.
func mountsForMinijail(pidFile string) ([]sysutil.MountInfo, error) {
	b, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", pidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", b)
	}
	return sysutil.MountInfoForPID(pid)
}

// arcMounts returns a list of mount point info for ARC's mount namespace.
func arcMounts() ([]sysutil.MountInfo, error) {
	pid, err := arc.InitPID()
	if err != nil {
		return nil, err
	}
	return sysutil.MountInfoForPID(int(pid))
}

// adbdMounts returns a list of mount point info for arc-adbd's mount namespace.
func adbdMounts(ctx context.Context) ([]sysutil.MountInfo, error) {
	goal, state, _, err := upstart.JobStatus(ctx, "arc-adbd")
	if err != nil {
		return nil, err
	}
	if goal == upstart.StopGoal && state == upstart.WaitingState {
		// On the current platform arc-adbd is not used.
		return nil, nil
	}
	return mountsForMinijail("/run/arc/adbd.pid")
}

// sdcardMounts returns a list of mount point info for sdcard.
// In ARC N, it returns a list of mount point info for sdcard daemon's mount
// namespace. In ARC P, it returns a list of esdfs typed mount points.
func sdcardMounts() ([]sysutil.MountInfo, error) {
	v, err := arc.SDKVersion()
	if err != nil {
		return nil, err
	}
	if v == arc.SDKN {
		// In ARC N, sdcard daemon is used. Return the mount points
		// in the mount namespace for the daemon.
		return mountsForMinijail("/run/arc/sdcard.pid")
	}

	// In ARC P, esdfs is used. Returns the mount points of esdfs file
	// system type.
	global, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return nil, err
	}
	var ret []sysutil.MountInfo
	for _, m := range global {
		if strings.HasPrefix(m.Fstype, "esdfs") {
			ret = append(ret, m)
		}
	}
	return ret, nil
}

// mountPassthroughMounts returns a list of mount point info for
// mount-passthrough daemons. Currently there are 8 mount-passthrough daemon
// jobs for MyFiles and removable media.
// The name should be matched to the following regexp:
//   "arc-(myfiles|removable-media)(-(default|read|write))?
func mountPassthroughMounts(ctx context.Context) ([]sysutil.MountInfo, error) {
	var ret []sysutil.MountInfo
	for _, job := range []string{
		"arc-myfiles",
		"arc-myfiles-default",
		"arc-myfiles-read",
		"arc-myfiles-write",
		"arc-removable-media",
		"arc-removable-media-default",
		"arc-removable-media-read",
		"arc-removable-media-write",
	} {
		_, _, pid, err := upstart.JobStatus(ctx, job)
		if err != nil {
			return nil, err
		}
		if pid == 0 {
			testing.ContextLogf(ctx, "%s is not running, skipped", job)
			continue
		}
		mounts, err := sysutil.MountInfoForPID(pid)
		if err != nil {
			return nil, err
		}
		ret = append(ret, mounts...)
	}
	return ret, nil
}

// obbMounts returns a list of mount point info for obb-mounter's namespace.
func obbMounts(ctx context.Context) ([]sysutil.MountInfo, error) {
	out, err := testexec.CommandContext(ctx, "pgrep", "-f", "-u", "root", "^/usr/bin/arc-obb-mounter").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse PID: %s", out)
	}
	return sysutil.MountInfoForPID(pid)
}

func joinMounts(mss ...[]sysutil.MountInfo) []sysutil.MountInfo {
	var ret []sysutil.MountInfo
	for _, ms := range mss {
		ret = append(ret, ms...)
	}
	return ret
}

func testNoARCLeak(ctx context.Context, s *testing.State, global []sysutil.MountInfo) {
	s.Log("Running testNoARCLeak")

	const root = "/opt/google/containers/android/rootfs/"

	var paths []string
	for _, m := range global {
		if !strings.HasPrefix(m.MountPath, root) {
			// Not the mount point under the container root.
			continue
		}
		p, err := filepath.Rel(root, m.MountPath)
		if err != nil {
			s.Errorf("Couldn't take relative path of %s from %s: %v", m.MountPath, root, err)
			return
		}
		// system/lib/arm is only required when houdini is used.
		if p == "system/lib/arm" {
			continue
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)

	expect := []string{
		"android-data",
		"root",
	}
	if !reflect.DeepEqual(paths, expect) {
		s.Errorf("Unexpected mount paths: got %v; want %v", paths, expect)
	}
}

func testNoARCSharedLeak(ctx context.Context, s *testing.State, arc, nonARC []sysutil.MountInfo) {
	s.Log("Running testNoARCSharedLeak")

	// Set of peer groups which are visible from outside of ARC container.
	visibles := make(map[int]struct{})
	for _, m := range nonARC {
		if m.Shared > 0 {
			visibles[m.Shared] = struct{}{}
		}
		if m.Master > 0 {
			visibles[m.Master] = struct{}{}
		}
	}

	// Peer groups in ARC container must not be visible from outside of
	// ARC container.
	for _, m := range arc {
		if m.Shared == 0 {
			// Not shared. Skip.
			continue
		}
		if _, ok := visibles[m.Shared]; ok {
			s.Errorf("Peer group in ARC container is being leaked: %s:%d", m.MountPath, m.Shared)
		}
	}
}

func testDebugfsTracefs(ctx context.Context, s *testing.State, arc []sysutil.MountInfo) {
	s.Log("Running testDebugfsTracefs")

	// If debugfs/tracefs is mounted somewhere in the container,
	// - It should be mounted at /sys/kernel/debug/tracing, and
	// - It should be /tracing portion of debugfs (or the root of tracefs for kernels >= 4.4)
	// And there is at most one such mount.
	// Or there could be sync debugfs mounted.
	// And there can be zero or one sync mounts.

	var numTracing, numSync int
	for _, m := range arc {
		switch m.Fstype {
		case "debugfs":
			if m.Root == "/tracing" && m.MountPath == "/sys/kernel/debug/tracing" {
				numTracing++
			} else if m.Root == "/sync" && m.MountPath == "/sys/kernel/debug/sync" {
				numSync++
			} else {
				s.Errorf("Unexpected debugfs mount point at %s", m.MountPath)
			}
		case "tracefs":
			if m.Root == "/" && m.MountPath == "/sys/kernel/debug/tracing" {
				numTracing++
			} else {
				s.Errorf("Unexpected tracefs mount point at %s", m.MountPath)
			}
		}
	}

	if numTracing != 1 {
		s.Errorf("Unexpected debugfs/tracefs mount points: got %d; want 1", numTracing)
	}
	if numSync != 0 && numSync != 1 {
		s.Errorf("Unexpected sync debug fs mount points: got %d; want 0 or 1", numSync)
	}
}

func testCgroup(ctx context.Context, s *testing.State, arc []sysutil.MountInfo) {
	s.Log("Running testCgroup")

	var paths []string
	for _, m := range arc {
		if m.Fstype != "cgroup" {
			continue
		}
		// This device exists only on some ARM boards like kevin.
		if m.MountPath == "/dev/stune" {
			continue
		}
		paths = append(paths, m.MountPath)
	}
	sort.Strings(paths)
	expect := []string{"/acct", "/dev/cpuctl", "/dev/cpuset"}
	if !reflect.DeepEqual(paths, expect) {
		s.Errorf("Unexpected cgroup paths: got %v; want %v", paths, expect)
		return
	}
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

func testCPUSet(ctx context.Context, s *testing.State, a *arc.ARC) {
	s.Log("Running testCPUSet")

	SDKVer, err := arc.SDKVersion()
	if err != nil {
		s.Error("Failed to find SDKVersion: ", err)
		return
	}

	CPUSpec := map[string]func(cpuset.CPUSet) bool{
		"foreground":        cpuset.Online().Equal,
		"top-app":           cpuset.Online().Equal,
		"background":        cpuset.Online().StrictSuperset,
		"system-background": cpuset.Online().StrictSuperset,
	}
	if SDKVer >= arc.SDKP {
		// In ARC P or later, restricted is added.
		CPUSpec["restricted"] = cpuset.Online().StrictSuperset
	}

	cpuset.CheckCPUSpec(s, CPUSpec)
}

func testADBD(ctx context.Context, s *testing.State, adbd []sysutil.MountInfo) {
	s.Log("Running testADBD")

	re := regexp.MustCompile(`^/run/arc/adbd(/ep[12])?$`)
	for _, m := range adbd {
		if m.Master > 0 {
			s.Error("adbd proxy container has unknown slave mount at ", m.MountPath)
			continue
		}
		if m.Shared == 0 {
			continue
		}
		if !re.MatchString(m.MountPath) {
			s.Error("adbd proxy container exposes unknown mount point at ", m.MountPath)
		}
	}
}

func testSDCard(ctx context.Context, s *testing.State, sdcard []sysutil.MountInfo) {
	s.Log("Running testSDCard")

	ver, err := arc.SDKVersion()
	if err != nil {
		s.Error("Failed to get SDK version: ", err)
		return
	}

	// If the mount point is shared it should be either:
	// - /mnt/runtime
	// - /mnt/runtime/{default,read,write}/$label
	// In ARC P, the following points are also shared:
	// - /run/arc/sdcard
	// - /run/arc/sdcard/{default,read,write}/$label
	// In ARC Q, the follow points are also shared:
	// - /run/arc/sdcard/full/$label
	pat := `^/mnt/runtime(/(default|read|write)/[^/]+)?$`
	if ver >= arc.SDKP {
		pat += `|^/run/arc/sdcard(/(default|read|write)/[^/]+)?$`
	}
	if ver >= arc.SDKQ {
		pat += `|^/run/arc/sdcard/full/[^/]+$`
	}
	re := regexp.MustCompile(pat)

	for _, m := range sdcard {
		if m.Master > 0 {
			s.Errorf("Unexpected SDCard slave mount at %s", m.MountPath)
			continue
		}
		if m.Shared == 0 {
			continue
		}
		if !re.MatchString(m.MountPath) {
			s.Errorf("Unexpected SDCard shared mount at %s", m.MountPath)
		}
	}
}

func testMountPassthrough(ctx context.Context, s *testing.State, mountPassthrough []sysutil.MountInfo) {
	s.Log("Running testMountPassthrough")

	for _, m := range mountPassthrough {
		// The only shared mount point is /mnt/dest.
		// Note that there might be multiple shared mount points at
		// the exactly same path.
		if m.Shared > 0 && m.MountPath != "/mnt/dest" {
			s.Errorf("Unexpected mount-passthrough shared mount at %s", m.MountPath)
		}
	}
}

func testOBBMount(ctx context.Context, s *testing.State, obb []sysutil.MountInfo) {
	s.Log("Running testOBBMount")

	for _, m := range obb {
		// The only shared mount point is /var/run/arc/obb.
		if m.Shared > 0 && m.MountPath != "/var/run/arc/obb" {
			s.Errorf("Unexpected OBB shared mount at %s", m.MountPath)
		}
	}
}

func testMountShared(ctx context.Context, s *testing.State, arcMs, adbd, sdcard, mountPassthrough, obb []sysutil.MountInfo) {
	ignored := make(map[string]struct{})
	if adbd == nil {
		// ADBD proxy container does not run on all boards because it
		// needs to have hardware and kernel support.
		ignored["/dev/usb-ffs/adb"] = struct{}{}
	}
	ver, err := arc.SDKVersion()
	if err != nil {
		s.Error("Failed to get SDK version: ", err)
		return
	}
	if ver >= arc.SDKP {
		// In ARC P, ignore initial tmpfs mount for /run/arc/sdcard
		// because it is slave mount but has the initns as its parent.
		ignored["/var/run/arc/sdcard"] = struct{}{}
	}
	if len(ignored) > 0 {
		var paths []string
		for p := range ignored {
			paths = append(paths, p)
		}
		s.Log("Ignored mount paths: ", paths)
	}

	peerGroups := make(map[int]struct{})
	for _, ms := range [][]sysutil.MountInfo{arcMs, adbd, sdcard, mountPassthrough, obb} {
		for _, m := range ms {
			if m.Shared > 0 {
				peerGroups[m.Shared] = struct{}{}
			}
		}
	}
	for _, m := range arcMs {
		if _, ok := ignored[m.MountPath]; ok {
			continue
		}
		// Masters of all non-allowed SLAVE mount points in ARC
		// container must be in containers.
		if m.Master == 0 {
			continue
		}
		if _, ok := peerGroups[m.Master]; !ok {
			s.Error("Unexpected slave mount at ", m.MountPath)
		}
	}
}

// RunTest exercises the ARC related mount point conditions.
func RunTest(ctx context.Context, s *testing.State, a *arc.ARC) {
	global, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		s.Fatal("Failed to get mountinfo list: ", err)
	}

	arc, err := arcMounts()
	if err != nil {
		s.Fatal("Failed to get mountinfo list for ARC: ", err)
	}

	adbd, err := adbdMounts(ctx)
	if err != nil {
		s.Fatal("Failed to get mountinfo list for arc-adbd: ", err)
	}

	sdcard, err := sdcardMounts()
	if err != nil {
		s.Fatal("Failed to get mountinfo list for sdcard: ", err)
	}

	mountPassthrough, err := mountPassthroughMounts(ctx)
	if err != nil {
		s.Fatal("Failed to get mountinfo list for mount-passthrough: ", err)
	}

	obb, err := obbMounts(ctx)
	if err != nil {
		s.Fatal("Failed to get mountinfo list for arc-obb-mounter: ", err)
	}

	testNoARCLeak(ctx, s, global)
	testNoARCSharedLeak(ctx, s, arc, joinMounts(global, adbd, sdcard, obb))
	testDebugfsTracefs(ctx, s, arc)
	testCgroup(ctx, s, arc)
	// TODO(hidehiko): This is not a part of "mount" tests. Find a good place to move.
	testCPUSet(ctx, s, a)
	testADBD(ctx, s, adbd)
	testSDCard(ctx, s, sdcard)
	testMountPassthrough(ctx, s, mountPassthrough)
	testOBBMount(ctx, s, obb)
	testMountShared(ctx, s, arc, adbd, sdcard, mountPassthrough, obb)
}
