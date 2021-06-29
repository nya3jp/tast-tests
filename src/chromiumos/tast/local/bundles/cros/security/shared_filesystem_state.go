// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/sandboxing"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SharedFilesystemState,
		Desc: "Reports on the state of the Chrome OS shared filesystem and fails if an unexpected mount is found when not logged in",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"nvaa@google.com",      // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// SharedFilesystemState test will fail if you are adding a new shared mount to
// the init mount namespace. If this is the case, follow these steps:
// 1. Confirm that it is necessary and prepare reasoning for why this mount must
//    be shared and in the init mount namespace.
// 2. Add the mount to the appropriate list below (based on whether it exists in
//    ARCVM/ARC++ and whether logged in or not).
// 3. Add short reasoning as a comment above the mount, then add a more detailed
//    explanation in
//    https://chrome-internal.googlesource.com/chromeos/docs/+/HEAD/security/shared_filesystem_state.md
// 4. Add nvaa or another chromeos-security engineer as a reviewer on the CL.
func SharedFilesystemState(ctx context.Context, s *testing.State) {
	// Names of processes whose children should be ignored. These processes themselves are also ignored.
	ignoredAncestorNames := make(map[string]struct{})
	for _, ancestorName := range sandboxing.IgnoredAncestors {
		ignoredAncestorNames[sandboxing.TruncateProcName(ancestorName)] = struct{}{}
	}

	if moblab.IsMoblab() {
		for _, moblabAncestorName := range sandboxing.IgnoredMoblabAncestors {
			ignoredAncestorNames[sandboxing.TruncateProcName(moblabAncestorName)] = struct{}{}
		}
	}

	exclusionsMap := make(map[string]struct{})
	for _, name := range sandboxing.Exclusions {
		exclusionsMap[sandboxing.TruncateProcName(name)] = struct{}{}
	}

	if upstart.JobExists(ctx, "ui") {
		s.Log("Restarting ui job to clean up stray processes")
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui job: ", err)
		}
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}

	if err := cryptohome.MountGuest(ctx); err != nil {
		s.Fatal("Failed to mount guest: ", err)
	}

	if err := sm.StartSession(ctx, cryptohome.GuestUser, ""); err != nil {
		s.Fatal("Failed to start guest session: ", err)
	}
	defer upstart.RestartJob(ctx, "ui")

	testType := "guest"
	testBody(s, testType, ignoredAncestorNames, exclusionsMap)

	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
	)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	testType = "arc-user"
	testBody(s, testType, ignoredAncestorNames, exclusionsMap)

}

func testBody(s *testing.State, testType string, ignoredAncestorNames, exclusionsMap map[string]struct{}) {
	// Additional info about all mounts and reasoning behind them can be found
	// here: go/shared-filesystem-state

	// BaseExpectedSharedMounts contains the paths of all shared mountpoints
	// that will be found on every system, whether ARC is enabled or not.
	BaseExpectedSharedMounts := map[string]bool{
		// This is where USB drives get mounted.
		"^/media$": true,
		// This is used to mount downloaded disk images.
		"^/run/imageloader$": true,
		// /run/netns is created implicitly by the ip netns command which is used
		// for network namespaces for ARC and for the proxy DNS service
		"^/run/netns$": true,
	}

	// ARCExpectedSharedMountsCommon contains the paths of all mountpoints that
	// are expected to be shared both when logged in as a guest and as a user on
	// an ARC++ device.
	ARCExpectedSharedMountsCommon := map[string]bool{
		// Contains the unix domain socket for Android Debugging
		// connection into the container.
		"^/run/arc/adb$": true,
	}

	// ARCExpectedSharedMountsUser contains the names of all mountpoints that are
	// expected to be shared when logged in as a user on an ARC++ device.
	ARCExpectedSharedMountsUser := map[string]bool{
		// Temporarily mounted to share cryptohome data with ARC++
		"^/run/arc/shared_mounts$": true,
		// Used to share Android's sdcard partition to ChromeOS
		"^/run/arc/sdcard$": true,
		// ARC OBB mounter uses /run/arc/obb to support Android OBB mount API.
		"^/run/arc/obb$": true,
		// This sets up the shadow user directory as a shared subtree.
		"^/home/\\.shadow/\\w+?/mount/user$": true,
		// /run/arc/media is used to share MyFiles and removable media with Android.
		// Many subdir mounts are created under /run/arc/media, and we want to include all of those as well.
		"^/run/arc/media(/|$)": true,
		// Used for Android Debugging over USB.
		"^/run/arc/adbd$": true,
		// These mount points are to ensure that the root
		// namespace's /home/root/<hash> is propagated into the mnt_concierge
		// namespace even when the latter namespace is created before login.
		"^/run/arcvm$": true,
		// See shared_mounts above.
		"^/run/arc/shared_mounts/data$": true,
		// See netns above.
		"^/run/netns/arc_netns$": true,
	}

	// ARCVMExpectedSharedMountsCommon contains the names of all mountpoints that
	// are expected to be shared both when logged in as a guest and also as a
	// user on an ARCVM device.
	ARCVMExpectedSharedMountsCommon := map[string]bool{
		// Used to share Android's sdcard partition to ChromeOS
		"^/run/arc/sdcard$": true,
		// These mount points are to ensure that the root
		// namespace's /home/root/<hash> is propagated into the mnt_concierge
		// namespace even when the latter namespace is created before login.
		"^/run/arcvm$": true,
	}

	// ARCVMExpectedSharedMountsUser contains the names of all mountpoints that are
	// expected to be shared when logged in as a user on an ARCVM device.
	ARCVMExpectedSharedMountsUser := map[string]bool{
		// This sets up the shadow user directory as a shared subtree.
		"^/home/\\.shadow/\\w+?/mount/user$": true,
		// See arcvm above.
		"^/run/arcvm/userhome$": true,
		// See sdcard above.
		"^/run/arc/sdcard/write/emulated$": true,
	}

	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to list running processes: ", err)
	}
	logName := testType + "-processes.txt"
	s.Logf("Writing %v processes to %v", len(procs), logName)
	lg, err := os.Create(filepath.Join(s.OutDir(), logName))
	if err != nil {
		s.Fatal("Failed to open log: ", err)
	}
	defer lg.Close()

	// We don't know that we'll see parent processes before their children (since PIDs can wrap around),
	// so do an initial pass to gather information.
	infos := make(map[int32]*sandboxing.ProcSandboxInfo)
	ignoredAncestorPIDs := make(map[int32]struct{})
	for _, proc := range procs {
		info, err := sandboxing.GetProcSandboxInfo(proc)
		// Even on error, write the partially-filled info to help in debugging.
		fmt.Fprintf(lg, "%5d %-15s uid=%-6d gid=%-6d pidns=%-10d mntns=%-10d nnp=%-5v seccomp=%-5v ecaps=%#x\n",
			proc.Pid, info.Name, info.Euid, info.Egid, info.PidNS, info.MntNS, info.NoNewPrivs, info.Seccomp, info.Ecaps)
		if err != nil {
			// An error could either indicate that the process exited or that we failed to parse /proc.
			// Check if the process is still there so we can report the error in the latter case.
			if status, serr := proc.Status(); serr == nil {
				s.Errorf("Failed to get info about process %d with status %q: %v", proc.Pid, status, err)
			}
			continue
		}

		infos[proc.Pid] = info

		// Determine if all of this process's children should also be ignored.
		_, ignoredByName := ignoredAncestorNames[info.Name]
		if ignoredByName ||
			// Assume that any executables under /usr/local are dev- or test-specific,
			// since /usr/local is mounted noexec if dev mode is disabled.
			strings.HasPrefix(info.Exe, "/usr/local/") ||
			// Autotest tests sometimes leave orphaned processes running after they exit,
			// so ignore anything that might e.g. be using a data file from /usr/local/autotest.
			strings.Contains(info.Cmdline, "autotest") {
			ignoredAncestorPIDs[proc.Pid] = struct{}{}
		}
	}

	// To determine which processes we expect, we need to know whether the
	// test device is running ARC++ or ARCVM and whether we are running as
	// guest or user.
	expectedSharedMounts := BaseExpectedSharedMounts
	var mountsCommon, mountsUser map[string]bool
	if arc.Supported() {
		if t, ok := arc.Type(); ok {
			switch t {
			case arc.Container:
				mountsCommon = ARCExpectedSharedMountsCommon
				mountsUser = ARCExpectedSharedMountsUser
			case arc.VM:
				mountsCommon = ARCVMExpectedSharedMountsCommon
				mountsUser = ARCVMExpectedSharedMountsUser
			default:
				s.Errorf("Unsupported ARC type %d", t)
			}

			for k, v := range mountsCommon {
				expectedSharedMounts[k] = v
			}
			if testType == "user" {
				for k, v := range mountsUser {
					expectedSharedMounts[k] = v
				}
			}
		} else {
			s.Error("Failed to detect ARC type")
		}
	}

	// We use the init process's info later to determine if other
	// processes have their own capabilities/namespaces or not.
	const initPID = 1
	initInfo := infos[initPID]
	if initInfo == nil {
		s.Fatal("Didn't find init process")
	}

	// We want to check to make sure that the only shared mounts in the
	// init mountns are the ones that we expect.
	const ignoreDaemonstore = "daemon-store"
	numInitChecked := 0
	var initSharedMounts []string
	var unexpectedInitSharedMounts []string
	for _, mountInfo := range initInfo.MountInfos {
		// Filter out mounts that we expect and would like to ignore.
		if strings.Contains(mountInfo.MountPoint, ignoreDaemonstore) {
			continue
		}

		for _, optField := range mountInfo.OptFields {
			// If it is a shared mount then we need to figure out
			// if it is expected or not
			if strings.Contains(optField, "shared") {
				initSharedMounts = append(initSharedMounts, mountInfo.MountPoint)
				isExpectedMount := false
				for mountName := range expectedSharedMounts {
					match, _ := regexp.MatchString(mountName, mountInfo.MountPoint)
					if match {
						isExpectedMount = true
						// Only delete from the map of mounts if it
						// is a full match. If it is a prefix match
						// (does not end with "$", leave it in the
						// map to be matched again.
						if mountName[len(mountName)-1:] == "$" {
							delete(expectedSharedMounts, mountName)
						}
						break
					}
				}
				if !isExpectedMount {
					unexpectedInitSharedMounts = append(unexpectedInitSharedMounts, mountInfo.MountPoint)
				}
			}
		}
		numInitChecked++
	}

	s.Logf("Checking status of %d processes", len(infos))
	numChecked := 0
	var haveOnlyMountsFlowingIn []string
	var haveSharedMounts []string
	var privProcsInInitMountNS []string
	var privProcsWithMountsFlowingIn []string

	for pid, info := range infos {
		if pid == initPID {
			continue
		}
		if _, ok := exclusionsMap[info.Name]; ok {
			continue
		}
		if _, ok := ignoredAncestorPIDs[pid]; ok {
			continue
		}
		if skip, err := sandboxing.ProcHasAncestor(pid, ignoredAncestorPIDs, infos); err == nil && skip {
			continue
		}

		if privileged(info) && info.MntNS == initInfo.MntNS {
			// Privileged processes running in the init mount namespace are exposed
			// to mounts created by other processes.
			privProcsInInitMountNS = append(privProcsInInitMountNS, info.Name)
			numChecked++
			continue
		}

		hasMountFlowingIn := false
		hasSharedMount := false
		for _, mountInfo := range info.MountInfos {
			for _, optField := range mountInfo.OptFields {
				// TODO(crbug/1207940): Remove blocked term here.
				if strings.Contains(optField, "master") {
					hasMountFlowingIn = true
					break
				}
				if strings.Contains(optField, "shared") {
					hasSharedMount = true
					break
				}
			}
		}

		if hasMountFlowingIn {
			haveOnlyMountsFlowingIn = append(haveOnlyMountsFlowingIn, info.Name)
		}
		if hasSharedMount {
			haveSharedMounts = append(haveSharedMounts, info.Name)
		}

		if (hasMountFlowingIn || hasSharedMount) && privileged(info) {
			privProcsWithMountsFlowingIn = append(privProcsWithMountsFlowingIn, info.Name)
		}

		numChecked++
	}

	s.Logf("Checked %d processes after exclusions", numChecked)
	s.Logf("%d processes have mounts flowing in", len(haveOnlyMountsFlowingIn))
	s.Logf("%d processes have shared mounts", len(haveSharedMounts))

	s.Logf("Checked %d mounts in init mount namespace", numInitChecked)
	s.Logf("%d mounts are shared", len(initSharedMounts))
	for _, mount := range initSharedMounts {
		s.Log(mount)
	}
	s.Logf("%d are unexpected shared mounts", len(unexpectedInitSharedMounts))
	for _, mount := range unexpectedInitSharedMounts {
		s.Log(mount)
	}
	s.Logf("%d expected shared mounts were not found", len(expectedSharedMounts))
	for mount := range expectedSharedMounts {
		s.Log(mount)
	}

	s.Logf("%d privileged processes are running in the init mount NS:", len(privProcsInInitMountNS))
	for _, proc := range privProcsInInitMountNS {
		s.Log(proc)
	}
	s.Logf("%d privileged processes are exposed to mount events:", len(privProcsWithMountsFlowingIn))
	for _, proc := range privProcsWithMountsFlowingIn {
		s.Log(proc)
	}

	sharedMountsLogName := testType + "-shared_mounts.txt"
	s.Log("Writing shared mounts to ", sharedMountsLogName)
	sharedMounts, err := os.Create(filepath.Join(s.OutDir(), sharedMountsLogName))
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer sharedMounts.Close()

	var expectedSharedMountsList []string
	for mount := range expectedSharedMounts {
		expectedSharedMountsList = append(expectedSharedMountsList, mount)
	}

	for _, l := range [][]string{initSharedMounts, expectedSharedMountsList, unexpectedInitSharedMounts} {
		b, err := json.Marshal(l)
		if err != nil {
			s.Error("Failed to marshal process list: ", err)
			continue
		}
		sharedMounts.Write(b)
		sharedMounts.WriteString("\n")
	}

	relevantProcessesLogName := testType + "-relevant_processes.txt"
	s.Log("Writing relevant processes to ", relevantProcessesLogName)
	relevantProcesses, err := os.Create(filepath.Join(s.OutDir(), relevantProcessesLogName))
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer relevantProcesses.Close()

	for _, l := range [][]string{haveOnlyMountsFlowingIn, haveSharedMounts} {
		b, err := json.Marshal(l)
		if err != nil {
			s.Error("Failed to marshal process list: ", err)
			continue
		}
		relevantProcesses.Write(b)
		relevantProcesses.WriteString("\n")
	}

	if len(unexpectedInitSharedMounts) > 0 && testType == "guest" {
		s.Error("Found unexpected shared mounts on the system when not logged in: ",
			unexpectedInitSharedMounts[0])
	}
}

func privileged(info *sandboxing.ProcSandboxInfo) bool {
	var privilegedCapIdxs = [...]int{
		21, // CAP_SYS_ADMIN
	}
	var privilegedCapMask uint64
	for _, idx := range privilegedCapIdxs {
		privilegedCapMask |= (1 << idx)
	}
	return info.Euid == 0 || (info.Ecaps&privilegedCapMask) > 0
}
