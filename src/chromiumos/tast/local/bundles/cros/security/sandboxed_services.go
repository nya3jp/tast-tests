// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/asan"
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
		{"lid_touchpad_helper", "root", "root", 0},
		{"thermal.sh", "root", "root", 0},
		{"daisydog", "watchdog", "watchdog", pidNS | mntNS | restrictCaps | noNewPrivs},
		{"permission_broker", "devbroker", "root", restrictCaps | noNewPrivs},
		{"netfilter-queue", "nfqueue", "nfqueue", restrictCaps | seccomp},
		{"anomaly_detector", "root", "syslog", 0},
		{"attestationd", "attestation", "attestation", restrictCaps | noNewPrivs | seccomp},
		{"periodic_scheduler", "root", "root", 0},
		{"metrics_client", "root", "root", 0},
		{"esif_ufd", "root", "root", 0},
		{"easy_unlock", "easy-unlock", "easy-unlock", 0},
		{"sslh-fork", "sslh", "sslh", pidNS | mntNS | restrictCaps | seccomp},
		{"upstart-socket-bridge", "root", "root", 0},
		{"timberslide", "root", "root", 0},
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
		{"arc-oemcrypto", "arc-oemcrypto", "arc-oemcrypto", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		{"udevadm", "root", "root", 0},
		{"usb_bouncer", "root", "root", 0},
		{"brcm_patchram_plus", "root", "root", 0},          // runs on some veyron boards
		{"rialto_cellular_autoconnect", "root", "root", 0}, // runs on veyron_rialto
		{"rialto_modem_watchdog", "root", "root", 0},       // runs on veyron_rialto
		{"netperf", "root", "root", 0},                     // started by Autotest tests
		{"tpm_managerd", "root", "root", 0},
		{"trunksd", "trunks", "trunks", restrictCaps | noNewPrivs | seccomp},
		{"imageloader", "root", "root", 0}, // uses NNP/seccomp but sometimes seen before sandboxing: https://crbug.com/936703#c16
		{"imageloader", "imageloaderd", "imageloaderd", mntNSNoPivotRoot | restrictCaps | noNewPrivs | seccomp},
		{"arc-networkd", "root", "root", noNewPrivs},
		{"arc-networkd", "arc-networkd", "arc-networkd", restrictCaps},

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
	}

	// exclusions contains names (from the "Name:" field in /proc/<pid>/status) of processes to ignore.
	exclusions := []string{
		"agetty",
		"aplay", // sometimes left behind by Autotest audio tests
		"autotest",
		"autotestd",
		"autotestd_monitor",
		"check_ethernet.hook",
		"chrome",
		"chrome-sandbox",
		"cras_test_client",
		"crash_reporter",
		"endpoint",
		"evemu-device",
		"flock",
		"grep",
		"init",
		"logger",
		"login",
		"mosys", // used to get system info: https://crbug.com/963888
		"nacl_helper",
		"nacl_helper_bootstrap",
		"nacl_helper_nonsfi",
		"ping",
		"ply-image",
		"ps",
		"recover_duts",
		"sleep",
		"sshd",
		"sudo",
		"tail",
		"timeout",
		"x11vnc",
		"bash", // TODO: check against script name instead
		"dash",
		"python",
		"python2",
		"python3",
		"python3.4",
		"python3.5",
		"python3.6",
		"python3.7",
		"run_oci", // used to run other processes
		"sh",
		"minijail0", // just launches other daemons; also runs as root to drop privs
		"minijail-init",
		"(agetty)", // initial name when systemd starts serial-getty; changes to "agetty" later
		"adb",      // sometimes appears on test images: https://crbug.com/792541
		"postinst", // runs cros_installer
	}

	// Per TASK_COMM_LEN, the kernel only uses 16 null-terminated bytes to hold process names
	// (which we later read from /proc/<pid>/status), so we shorten names in the baseline and exclusion list.
	// See https://stackoverflow.com/questions/23534263 for more discussion.
	//
	// Using "Name:" from /status matches what the Autotest test was doing, but it can lead to unexpected collisions.
	// /exe is undesirable since executables like /usr/bin/coreutils implement many commands.
	// /cmdline may be modified by the process.
	const maxProcNameLen = 15
	truncateProcName := func(s string) string {
		if len(s) <= maxProcNameLen {
			return s
		}
		return s[:maxProcNameLen]
	}

	// Names of processes whose children should be ignored. These processes themselves are also ignored.
	ignoredAncestorNames := map[string]struct{}{
		truncateProcName("kthreadd"):           {}, // kernel processes
		truncateProcName("local_test_runner"):  {}, // Tast-related processes
		truncateProcName("periodic_scheduler"): {}, // runs cron scripts
		truncateProcName("arc-setup"):          {}, // runs patchoat and other Android programs
		truncateProcName("cros_installer"):     {}, // runs during system updates
		truncateProcName("python2.7"):          {}, // stale Autotest processes: https://crbug.com/936703#c39
		truncateProcName("dev_debug_vboot"):    {}, // executed by chromeos-setgoodkernel: https://crbug.com/962134
	}
	if moblab.IsMoblab() {
		ignoredAncestorNames[truncateProcName("apache2")] = struct{}{} // serves UI and runs other procs: https://crbug.com/962137
	}

	baselineMap := make(map[string][]*procReqs, len(baseline))
	for _, reqs := range baseline {
		name := truncateProcName(reqs.name)
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
	for _, name := range exclusions {
		exclusionsMap[truncateProcName(name)] = struct{}{}
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
	infos := make(map[int32]*procSandboxInfo)
	ignoredAncestorPIDs := make(map[int32]struct{})
	for _, proc := range procs {
		info, err := getProcSandboxInfo(proc)
		// Even on error, write the partially-filled info to help in debugging.
		fmt.Fprintf(lg, "%5d %-15s uid=%-6d gid=%-6d pidns=%-10d mntns=%-10d nnp=%-5v seccomp=%-5v ecaps=%#x\n",
			proc.Pid, info.name, info.euid, info.egid, info.pidNS, info.mntNS, info.noNewPrivs, info.seccomp, info.ecaps)
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
		_, ignoredByName := ignoredAncestorNames[info.name]
		if ignoredByName ||
			// Assume that any executables under /usr/local are dev- or test-specific,
			// since /usr/local is mounted noexec if dev mode is disabled.
			strings.HasPrefix(info.exe, "/usr/local/") ||
			// Autotest tests sometimes leave orphaned processes running after they exit,
			// so ignore anything that might e.g. be using a data file from /usr/local/autotest.
			strings.Contains(info.cmdline, "autotest") {
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
		if _, ok := exclusionsMap[info.name]; ok {
			continue
		}
		if _, ok := ignoredAncestorPIDs[pid]; ok {
			continue
		}
		if skip, err := procHasAncestor(pid, ignoredAncestorPIDs, infos); err == nil && skip {
			continue
		}

		numChecked++

		// We may have expectations for multiple users in the case of a process that forks and drops privileges.
		var reqs *procReqs
		var reqUID uint32
		for _, r := range baselineMap[info.name] {
			uid, err := parseID(r.euser, sysutil.GetUID)
			if err != nil {
				s.Errorf("Failed to look up user %q for PID %v", r.euser, pid)
				continue
			}
			// Favor reqs that exactly match the process's EUID, but fall back to the first one we see.
			match := uid == info.euid
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
			if info.euid == 0 {
				s.Errorf("Unexpected %q process %v (%v) running as root", info.name, pid, info.exe)
			}
			continue
		}

		var problems []string

		if info.euid != reqUID {
			problems = append(problems, fmt.Sprintf("effective UID %v; want %v", info.euid, reqUID))
		}

		if gid, err := parseID(reqs.egroup, sysutil.GetGID); err != nil {
			s.Errorf("Failed to look up group %q for PID %v", reqs.egroup, pid)
		} else if info.egid != gid {
			problems = append(problems, fmt.Sprintf("effective GID %v; want %v", info.egid, gid))
		}

		// We test for PID/mount namespaces and capabilities by comparing against what init is using
		// since processes inherit these by default.
		if reqs.features&pidNS != 0 && info.pidNS != -1 && info.pidNS == initInfo.pidNS {
			problems = append(problems, "missing PID namespace")
		}
		if reqs.features&(mntNS|mntNSNoPivotRoot) != 0 && info.mntNS != -1 && info.mntNS == initInfo.mntNS {
			problems = append(problems, "missing mount namespace")
		}
		if reqs.features&restrictCaps != 0 && info.ecaps == initInfo.ecaps {
			problems = append(problems, "no restricted capabilities")
		}
		if reqs.features&noNewPrivs != 0 && !info.noNewPrivs {
			problems = append(problems, "missing no_new_privs")
		}
		// Minijail disables seccomp at runtime when ASan is enabled, so don't check it in that case.
		if reqs.features&seccomp != 0 && !info.seccomp && !asanEnabled {
			problems = append(problems, "seccomp filter disabled")
		}

		// If a mount namespace is required and used, but some of the init process's test image mounts
		// are still present, then the process didn't call pivot_root().
		if reqs.features&mntNS != 0 && info.mntNS != -1 && info.mntNS != initInfo.mntNS && info.hasTestImageMounts {
			problems = append(problems, "did not call pivot_root(2)")
		}

		if len(problems) > 0 {
			s.Errorf("%q process %v (%v) isn't properly sandboxed: %s",
				info.name, pid, info.exe, strings.Join(problems, ", "))
		}
	}

	s.Logf("Checked %d processes after exclusions", numChecked)
}

// procSandboxInfo holds sandboxing-related information about a running process.
type procSandboxInfo struct {
	name               string // "Name:" value from /proc/<pid>/status
	exe                string // full executable path
	cmdline            string // space-separated command line
	ppid               int32  // parent PID
	euid, egid         uint32 // effective UID and GID
	pidNS, mntNS       int64  // PID and mount namespace IDs (-1 if unknown)
	ecaps              uint64 // effective capabilities
	noNewPrivs         bool   // no_new_privs is set (see "minijail -N")
	seccomp            bool   // seccomp filter is active
	hasTestImageMounts bool   // has test-image-only mounts
}

// getProcSandboxInfo returns sandboxing-related information about proc.
// An error is returned if any files cannot be read or if malformed data is encountered,
// but the partially-filled info is still returned.
func getProcSandboxInfo(proc *process.Process) (*procSandboxInfo, error) {
	var info procSandboxInfo
	var firstErr error
	saveErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	// Ignore errors for e.g. kernel processes.
	info.exe, _ = proc.Exe()
	info.cmdline, _ = proc.Cmdline()

	var err error
	if info.ppid, err = proc.Ppid(); err != nil {
		saveErr(errors.Wrap(err, "failed to get parent"))
	}

	if uids, err := proc.Uids(); err != nil {
		saveErr(errors.Wrap(err, "failed to get UIDs"))
	} else {
		info.euid = uint32(uids[1])
	}

	if gids, err := proc.Gids(); err != nil {
		saveErr(errors.Wrap(err, "failed to get GIDs"))
	} else {
		info.egid = uint32(gids[1])
	}

	// Namespace data appears to sometimes be missing for (exiting?) processes: https://crbug.com/936703
	if info.pidNS, err = readProcNamespace(proc.Pid, "pid"); os.IsNotExist(err) && proc.Pid != 1 {
		info.pidNS = -1
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed to read pid namespace"))
	}
	if info.mntNS, err = readProcNamespace(proc.Pid, "mnt"); os.IsNotExist(err) && proc.Pid != 1 {
		info.mntNS = -1
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed to read mnt namespace"))
	}

	// Read additional info from /proc/<pid>/status.
	status, err := readProcStatus(proc.Pid)
	if err != nil {
		saveErr(errors.Wrap(err, "failed reading status"))
	} else {
		if info.ecaps, err = strconv.ParseUint(status["CapEff"], 16, 64); err != nil {
			saveErr(errors.Wrapf(err, "failed parsing effective caps %q", status["CapEff"]))
		}
		info.name = status["Name"]
		info.noNewPrivs = status["NoNewPrivs"] == "1"
		info.seccomp = status["Seccomp"] == "2" // 1 is strict, 2 is filter
	}

	// Check whether any mounts that only occur in test images are available to the process.
	// These are limited to the init mount namespace, so if a process has its own namespace,
	// it shouldn't have these (assuming that it called pivot_root()).
	if mnts, err := readProcMountpoints(proc.Pid); os.IsNotExist(err) || err == syscall.EINVAL {
		// mounts files are sometimes missing or unreadable: https://crbug.com/936703#c14
	} else if err != nil {
		saveErr(errors.Wrap(err, "failed reading mountpoints"))
	} else {
		for _, mnt := range mnts {
			for _, tm := range []string{"/usr/local", "/var/db/pkg", "/var/lib/portage"} {
				if mnt == tm {
					info.hasTestImageMounts = true
					break
				}
			}
		}
	}

	return &info, firstErr
}

// readProcNamespace returns pid's namespace ID for name (e.g. "pid" or "mnt"),
// per /proc/<pid>/ns/<name>. This may return os.ErrNotExist: https://crbug.com/936703
func readProcNamespace(pid int32, name string) (int64, error) {
	v, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", pid, name))
	if err != nil {
		return -1, err
	}
	// The link value should have the form ":[<id>]"
	pre := name + ":["
	suf := "]"
	if !strings.HasPrefix(v, pre) || !strings.HasSuffix(v, suf) {
		return -1, errors.Errorf("unexpected value %q", v)
	}
	return strconv.ParseInt(v[len(pre):len(v)-len(suf)], 10, 64)
}

// readProcMountpoints returns all mountpoints listed in /proc/<pid>/mounts.
// This may return os.ErrNotExist or syscall.EINVAL for zombie processes: https://crbug.com/936703
func readProcMountpoints(pid int32) ([]string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/mounts", pid))
	// ioutil.ReadFile can return an *os.PathError. If it's os.ErrNotExist, we return it directly
	// since it's easy to check, but for other errors, we return the inner error (which is a syscall.Errno)
	// so that callers can inspect it.
	if pathErr, ok := err.(*os.PathError); ok && !os.IsNotExist(err) {
		return nil, pathErr.Err
	} else if err != nil {
		return nil, err
	}
	var mounts []string
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if ln == "" {
			continue
		}
		// Example line:
		// run /var/run tmpfs rw,seclabel,nosuid,nodev,noexec,relatime,mode=755 0 0
		parts := strings.Fields(ln)
		if len(parts) != 6 {
			return nil, errors.Errorf("failed to parse line %q", ln)
		}
		mounts = append(mounts, parts[1])
	}
	return mounts, nil
}

// procStatusLineRegexp is used to split a line from /proc/<pid>/status. Example content:
// Name:	powerd
// State:	S (sleeping)
// Tgid:	1249
// ...
var procStatusLineRegexp = regexp.MustCompile(`^([^:]+):\t(.*)$`)

// readProcStatus parses /proc/<pid>/status and returns its key/value pairs.
func readProcStatus(pid int32) (map[string]string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, err
	}

	vals := make(map[string]string)
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		// Skip blank lines: https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1772671
		if ln == "" {
			continue
		}
		ms := procStatusLineRegexp.FindStringSubmatch(ln)
		if ms == nil {
			return nil, errors.Errorf("failed to parse line %q", ln)
		}
		vals[ms[1]] = ms[2]
	}
	return vals, nil
}

// procHasAncestor returns true if pid has any of ancestorPIDs as an ancestor process.
// infos should contain the full set of processes and is used to look up data.
func procHasAncestor(pid int32, ancestorPIDs map[int32]struct{},
	infos map[int32]*procSandboxInfo) (bool, error) {
	info, ok := infos[pid]
	if !ok {
		return false, errors.Errorf("process %d not found", pid)
	}

	for {
		pinfo, ok := infos[info.ppid]
		if !ok {
			return false, errors.Errorf("parent process %d not found", info.ppid)
		}
		if _, ok := ancestorPIDs[info.ppid]; ok {
			return true, nil
		}
		if info.ppid == 1 {
			return false, nil
		}
		info = pinfo
	}
}
