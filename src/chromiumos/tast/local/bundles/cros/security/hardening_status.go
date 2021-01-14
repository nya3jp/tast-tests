// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/bundles/cros/security/sandboxing"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HardeningStatus,
		Desc: "Report on status of Chrome OS hardening efforts",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{},
	})
}

func HardeningStatus(ctx context.Context, s *testing.State) {
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

	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to list running processes: ", err)
	}
	const logName = "processes.txt"
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

	// We use the init process's info later to determine if other
	// processes have their own capabilities/namespaces or not.
	const initPID = 1
	initInfo := infos[initPID]
	if initInfo == nil {
		s.Fatal("Didn't find init process")
	}

	s.Logf("Checking status of %d processes", len(infos))
	numChecked := 0
	numFlowingIn := 0
	numShared := 0
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

		hasMountFlowingIn := false
		hasSharedMount := false
		for _, mountInfo := range info.MountInfos {
			for _, optField := range mountInfo.OptFields {
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
			numFlowingIn++
		}
		if hasSharedMount {
			numShared++
		}
		numChecked++
	}

	s.Logf("Checked %d processes after exclusions", numChecked)
	s.Logf("%d processes have mounts flowing in", numFlowingIn)
	s.Logf("%d processes have shared mounts", numShared)
}
