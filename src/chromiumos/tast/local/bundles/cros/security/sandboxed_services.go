// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/asan"
	"chromiumos/tast/local/bundles/cros/security/sandboxing"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SandboxedServices,
		Desc: "Verify running processes' sandboxing status against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func SandboxedServices(ctx context.Context, s *testing.State) {
	type feature int // security feature that may be set on a process
	const (
		pidNS            feature = 1 << iota // process runs in unique PID namespace
		mntNS                                // process runs in unique mount namespace with pivot_root(2)
		mntNSNoPivotRoot                     // like mntNS, but pivot_root() not required
		restrictCaps                         // process runs with restricted capabilities
		noNewPrivs                           // process runs with no_new_privs set (see "minijail -N")
		seccomp                              // process runs with a seccomp filter
	)

	// procReqs holds sandboxing requirements for a process.
	type procReqs struct {
		name          string  // process name from "Name:" in /proc/<pid>/status (long names will be truncated)
		euser, egroup string  // effective user and group (either username or numeric ID)
		features      feature // bitfield of security features enabled for the process
	}

	// baseline maps from process names (from the "Name:" field in /proc/<pid>/status)
	// to expected sandboxing features. Every root process must be listed here; non-root process will
	// also be checked if listed. Other non-root processes, and entries listed here that aren't running,
	// will be ignored. A single process name may be listed multiple times with different users.
	baseline := []*procReqs{
		{"udevd", "root", "root", 0},  // needs root to create device nodes and change owners/perms
		{"frecon", "root", "root", 0}, // needs root and no namespacing to launch shells
		{"session_manager", "root", "root", 0},
		{"rsyslogd", "syslog", "syslog", mntNS | restrictCaps},
		{"systemd-journal", "syslog", "syslog", mntNS | restrictCaps},
		{"dbus-daemon", "messagebus", "messagebus", restrictCaps},
		{"wpa_supplicant", "wpa", "wpa", restrictCaps | noNewPrivs},
		{"shill", "shill", "shill", restrictCaps | noNewPrivs},
		{"chapsd", "chaps", "chronos-access", restrictCaps | noNewPrivs},
		{"cryptohomed", "root", "root", 0},
		{"cryptohome-namespace-mounter", "root", "root", 0},
		{"powerd", "power", "power", restrictCaps},
		{"ModemManager", "modem", "modem", restrictCaps | noNewPrivs},
		{"dhcpcd", "dhcp", "dhcp", restrictCaps},
		{"memd", "root", "root", pidNS | mntNS | noNewPrivs | seccomp},
		{"metrics_daemon", "metrics", "metrics", 0},
		{"disks", "cros-disks", "cros-disks", restrictCaps | noNewPrivs},
		{"update_engine", "root", "root", 0},
		{"update_engine_client", "root", "root", 0},
		{"bluetoothd", "bluetooth", "bluetooth", restrictCaps | noNewPrivs},
		{"debugd", "root", "root", mntNS},
		{"cras", "cras", "cras", mntNS | restrictCaps | noNewPrivs},
		{"tcsd", "tss", "tss", restrictCaps},
		{"mtpd", "mtp", "mtp", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"tlsdated", "tlsdate", "tlsdate", restrictCaps},
		{"tlsdated-setter", "root", "root", noNewPrivs | seccomp},
		{"thermal.sh", "root", "root", 0},
		{"daisydog", "watchdog", "watchdog", pidNS | mntNS | restrictCaps | noNewPrivs},
		{"permission_broker", "devbroker", "root", restrictCaps | noNewPrivs},
		{"netfilter-queue", "nfqueue", "nfqueue", restrictCaps | seccomp},
		{"anomaly_detector", "root", "syslog", 0},
		{"attestationd", "attestation", "attestation", restrictCaps | noNewPrivs | seccomp},
		{"pca_agentd", "attestation", "attestation", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"periodic_scheduler", "root", "root", 0},
		{"metrics_client", "root", "root", 0},
		{"esif_ufd", "root", "root", 0},
		{"easy_unlock", "easy-unlock", "easy-unlock", 0},
		{"sslh-fork", "sslh", "sslh", pidNS | mntNS | restrictCaps | seccomp},
		{"upstart-socket-bridge", "root", "root", 0},
		{"timberslide", "root", "root", 0},
		{"timberslide-watcher.sh", "root", "root", 0},
		{"auditd", "root", "root", 0},
		{"firewalld", "firewall", "firewall", pidNS | mntNS | restrictCaps | noNewPrivs},
		{"conntrackd", "nfqueue", "nfqueue", mntNS | restrictCaps | noNewPrivs | seccomp},
		{"avahi-daemon", "avahi", "avahi", restrictCaps},
		{"upstart-udev-bridge", "root", "root", 0},
		{"midis", "midis", "midis", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"bio_crypto_init", "biod", "biod", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"biod", "biod", "biod", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"cros_camera_service", "arc-camera", "arc-camera", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"cros_camera_algo", "arc-camera", "arc-camera", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"arc_camera_service", "arc-camera", "arc-camera", restrictCaps},
		{"arc-obb-mounter", "root", "root", pidNS | mntNS},
		{"cdm-oemcrypto", "cdm-oemcrypto", "cdm-oemcrypto", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"udevadm", "root", "root", 0},
		{"usb_bouncer", "root", "root", 0},
		{"brcm_patchram_plus", "root", "root", 0},          // runs on some veyron boards
		{"os_install_service", "root", "root", 0},          // runs on reven
		{"rialto_cellular_autoconnect", "root", "root", 0}, // runs on veyron_rialto
		{"rialto_modem_watchdog", "root", "root", 0},       // runs on veyron_rialto
		{"netperf", "root", "root", 0},                     // started by Autotest tests
		{"tpm_managerd", "root", "root", 0},
		{"trunksd", "trunks", "trunks", restrictCaps | noNewPrivs | seccomp},
		{"imageloader", "root", "root", 0}, // uses NNP/seccomp but sometimes seen before sandboxing: https://crbug.com/936703#c16
		{"imageloader", "imageloaderd", "imageloaderd", mntNSNoPivotRoot | restrictCaps | noNewPrivs | seccomp},
		{"patchpaneld", "root", "root", noNewPrivs},
		{"patchpaneld", "patchpaneld", "patchpaneld", restrictCaps},
		{"cros_healthd", "root", "root", mntNS},                                                       // cros_healthd's root-level executor
		{"cros_healthd", "cros_healthd", "cros_healthd", mntNS | restrictCaps | noNewPrivs | seccomp}, // main cros_healthd daemon

		// These processes run as root in the ARC container.
		{"app_process", "android-root", "android-root", pidNS | mntNS},
		{"debuggerd", "android-root", "android-root", pidNS | mntNS},
		{"debuggerd:sig", "android-root", "android-root", pidNS | mntNS},
		{"healthd", "android-root", "android-root", pidNS | mntNS},
		{"vold", "android-root", "android-root", pidNS | mntNS},

		// These processes run as non-root in the ARC container.
		{"boot_latch", "656360", "656360", pidNS | mntNS | restrictCaps},
		{"bugreportd", "657360", "656367", pidNS | mntNS | restrictCaps},
		{"logd", "656396", "656396", pidNS | mntNS | restrictCaps},
		{"servicemanager", "656360", "656360", pidNS | mntNS | restrictCaps},
		{"surfaceflinger", "656360", "656363", pidNS | mntNS | restrictCaps},

		// Small, one-off init/setup scripts that don't spawn daemons and that are short-lived.
		{"activate_date.service", "root", "root", 0},
		{"chromeos-trim", "root", "root", 0},
		{"crx-import.sh", "root", "root", 0},
		{"dump_vpd_log", "root", "root", 0},
		{"lockbox-cache.sh", "root", "root", 0},
		{"powerd-pre-start.sh", "root", "root", 0},
		{"update_rw_vpd", "root", "root", 0},
		{"vpd_get_value", "root", "root", 0},
		{"vpd_icc", "root", "root", 0},

		// One-off processes that we see when this test runs together with other tests.
		// src/overlays/overlay-kip/chromeos-base/modem-watchdog/files/chromeos-kip-modem-watchdog.sh
		{"chromeos-kip-modem-watchdog.sh", "root", "root", 0},
		// src/platform2/installer/chromeos-setgoodkernel
		{"chromeos-setgoodkernel", "root", "root", 0},
		{"dbus-send", "root", "root", 0},
		// src/third_party/flashrom/
		{"flashrom", "root", "root", 0},
		// src/platform/factory/sh/goofy_control.sh
		{"goofy_control.sh", "root", "root", 0},
		// src/third_party/chromiumos-overlay/sys-apps/ureadahead/files/init/ureadahead.conf
		{"ureadahead", "root", "root", 0},
		{"sed", "root", "root", 0},
		{"start", "root", "root", 0},
	}

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

	baselineMap := make(map[string][]*procReqs, len(baseline))
	for _, reqs := range baseline {
		name := sandboxing.TruncateProcName(reqs.name)
		baselineMap[name] = append(baselineMap[name], reqs)
	}
	for name, rs := range baselineMap {
		users := make(map[string]struct{}, len(rs))
		for _, r := range rs {
			if _, ok := users[r.euser]; ok {
				s.Fatalf("Duplicate %q requirements for user %q in baseline", name, r.euser)
			}
			users[r.euser] = struct{}{}
		}
	}

	exclusionsMap := make(map[string]struct{})
	for _, name := range sandboxing.Exclusions {
		exclusionsMap[sandboxing.TruncateProcName(name)] = struct{}{}
	}

	// parseID first tries to parse str (a procReqs euser or egroup field) as a number.
	// Failing that, it passes it to lookup, which should be sysutil.GetUID or sysutil.GetGID.
	parseID := func(str string, lookup func(string) (uint32, error)) (uint32, error) {
		if id, err := strconv.Atoi(str); err == nil {
			return uint32(id), nil
		}
		if id, err := lookup(str); err == nil {
			return id, nil
		}
		return 0, errors.New("couldn't parse as number and lookup failed")
	}

	if upstart.JobExists(ctx, "ui") {
		s.Log("Restarting ui job to clean up stray processes")
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui job: ", err)
		}
	}

	asanEnabled, err := asan.Enabled(ctx)
	if err != nil {
		s.Error("Failed to check if ASan is enabled: ", err)
	} else if asanEnabled {
		s.Log("ASan is enabled; will skip seccomp checks")
	}

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

	s.Logf("Comparing %d processes against baseline", len(infos))
	numChecked := 0
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

		numChecked++

		// We may have expectations for multiple users in the case of a process that forks and drops privileges.
		var reqs *procReqs
		var reqUID uint32
		for _, r := range baselineMap[info.Name] {
			uid, err := parseID(r.euser, sysutil.GetUID)
			if err != nil {
				s.Errorf("Failed to look up user %q for PID %v", r.euser, pid)
				continue
			}
			// Favor reqs that exactly match the process's EUID, but fall back to the first one we see.
			match := uid == info.Euid
			if match || reqs == nil {
				reqs = r
				reqUID = uid
				if match {
					break
				}
			}
		}

		if reqs == nil {
			// Processes running as root must always be listed in the baseline.
			// We ignore unlisted non-root processes on the assumption that they've already done some sandboxing.
			if info.Euid == 0 {
				s.Errorf("Unexpected %q process %v (%v) running as root", info.Name, pid, info.Exe)
			}
			continue
		}

		var problems []string

		if info.Euid != reqUID {
			problems = append(problems, fmt.Sprintf("effective UID %v; want %v", info.Euid, reqUID))
		}

		if gid, err := parseID(reqs.egroup, sysutil.GetGID); err != nil {
			s.Errorf("Failed to look up group %q for PID %v", reqs.egroup, pid)
		} else if info.Egid != gid {
			problems = append(problems, fmt.Sprintf("effective GID %v; want %v", info.Egid, gid))
		}

		// We test for PID/mount namespaces and capabilities by comparing against what init is using
		// since processes inherit these by default.
		if reqs.features&pidNS != 0 && info.PidNS != -1 && info.PidNS == initInfo.PidNS {
			problems = append(problems, "missing PID namespace")
		}
		if reqs.features&(mntNS|mntNSNoPivotRoot) != 0 && info.MntNS != -1 && info.MntNS == initInfo.MntNS {
			problems = append(problems, "missing mount namespace")
		}
		if reqs.features&restrictCaps != 0 && info.Ecaps == initInfo.Ecaps {
			problems = append(problems, "no restricted capabilities")
		}
		if reqs.features&noNewPrivs != 0 && !info.NoNewPrivs {
			problems = append(problems, "missing no_new_privs")
		}
		// Minijail disables seccomp at runtime when ASan is enabled, so don't check it in that case.
		if reqs.features&seccomp != 0 && !info.Seccomp && !asanEnabled {
			problems = append(problems, "seccomp filter disabled")
		}

		// If a mount namespace is required and used, but some of the init process's test image mounts
		// are still present, then the process didn't call pivot_root().
		if reqs.features&mntNS != 0 && info.MntNS != -1 && info.MntNS != initInfo.MntNS && info.HasTestImageMounts {
			problems = append(problems, "did not call pivot_root(2)")
		}

		if len(problems) > 0 {
			s.Errorf("%q process %v (%v) isn't properly sandboxed: %s",
				info.Name, pid, info.Exe, strings.Join(problems, ", "))
		}
	}

	s.Logf("Checked %d processes after exclusions", numChecked)
}
