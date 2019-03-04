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

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/asan"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SandboxedServices,
		Desc: "Verify running processes' sandboxing status against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"informational"},
	})
}

func SandboxedServices(ctx context.Context, s *testing.State) {
	type feature int // security feature that may be set on a process
	const (
		pidNS        feature = 1 << iota // process runs in unique PID namespace
		mntNS                            // process runs in unique mount namespace with pivot_root(2)
		restrictCaps                     // process runs with restricted capabilities
		noNewPrivs                       // process runs with no_new_privs set (see "minijail -N")
		seccomp                          // process runs with a seccomp filter
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
		&procReqs{"udevd", "root", "root", 0},  // needs root to create device nodes and change owners/perms
		&procReqs{"frecon", "root", "root", 0}, // needs root and no namespacing to launch shells
		&procReqs{"session_manager", "root", "root", 0},
		&procReqs{"rsyslogd", "syslog", "syslog", mntNS | restrictCaps},
		&procReqs{"systemd-journal", "syslog", "syslog", mntNS | restrictCaps},
		&procReqs{"dbus-daemon", "messagebus", "messagebus", restrictCaps},
		&procReqs{"wpa_supplicant", "wpa", "wpa", restrictCaps | noNewPrivs},
		&procReqs{"shill", "shill", "shill", restrictCaps | noNewPrivs},
		&procReqs{"chapsd", "chaps", "chronos-access", restrictCaps | noNewPrivs},
		&procReqs{"cryptohomed", "root", "root", 0},
		&procReqs{"powerd", "power", "power", restrictCaps},
		&procReqs{"ModemManager", "modem", "modem", restrictCaps | noNewPrivs},
		&procReqs{"dhcpcd", "dhcp", "dhcp", restrictCaps},
		&procReqs{"memd", "root", "root", pidNS | mntNS | noNewPrivs | seccomp},
		&procReqs{"metrics_daemon", "root", "root", 0},
		&procReqs{"disks", "cros-disks", "cros-disks", restrictCaps | noNewPrivs},
		&procReqs{"update_engine", "root", "root", 0},
		&procReqs{"bluetoothd", "bluetooth", "bluetooth", restrictCaps | noNewPrivs},
		&procReqs{"debugd", "root", "root", mntNS},
		&procReqs{"cras", "cras", "cras", mntNS | restrictCaps | noNewPrivs},
		&procReqs{"tcsd", "tss", "root", restrictCaps},
		&procReqs{"cromo", "cromo", "cromo", 0},
		&procReqs{"wimax-manager", "root", "root", 0},
		&procReqs{"mtpd", "mtp", "mtp", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"tlsdated", "tlsdate", "tlsdate", restrictCaps},
		&procReqs{"tlsdated-setter", "root", "root", noNewPrivs | seccomp},
		&procReqs{"lid_touchpad_helper", "root", "root", 0},
		&procReqs{"thermal.sh", "root", "root", 0},
		&procReqs{"daisydog", "watchdog", "watchdog", pidNS | mntNS | restrictCaps | noNewPrivs},
		&procReqs{"permission_broker", "devbroker", "root", restrictCaps | noNewPrivs},
		&procReqs{"netfilter-queue", "nfqueue", "nfqueue", restrictCaps | seccomp},
		&procReqs{"anomaly_collector", "root", "root", 0},
		&procReqs{"attestationd", "attestation", "attestation", restrictCaps | noNewPrivs | seccomp},
		&procReqs{"periodic_scheduler", "root", "root", 0},
		&procReqs{"esif_ufd", "root", "root", 0},
		&procReqs{"easy_unlock", "easy-unlock", "easy-unlock", 0},
		&procReqs{"sslh-fork", "sslh", "sslh", pidNS | mntNS | restrictCaps | seccomp},
		&procReqs{"upstart-socket-bridge", "root", "root", 0},
		&procReqs{"timberslide", "root", "root", 0},
		&procReqs{"firewalld", "firewall", "firewall", pidNS | mntNS | restrictCaps | noNewPrivs},
		&procReqs{"conntrackd", "nfqueue", "nfqueue", mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"avahi-daemon", "avahi", "avahi", restrictCaps},
		&procReqs{"upstart-udev-bridge", "root", "root", 0},
		&procReqs{"midis", "midis", "midis", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"bio_crypto_init", "biod", "biod", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"biod", "biod", "biod", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"cros_camera_service", "arc-camera", "arc-camera", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"cros_camera_algo", "arc-camera", "arc-camera", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"arc_camera_service", "arc-camera", "arc-camera", restrictCaps},
		&procReqs{"arc-obb-mounter", "root", "root", pidNS | mntNS},
		&procReqs{"arc-oemcrypto", "arc-oemcrypto", "arc-oemcrypto", pidNS | mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"brcm_patchram_plus", "root", "root", 0}, // runs on some veyron boards
		&procReqs{"tpm_managerd", "root", "root", 0},
		&procReqs{"trunksd", "trunks", "trunks", restrictCaps | noNewPrivs | seccomp},
		&procReqs{"imageloader", "root", "root", noNewPrivs | seccomp},
		&procReqs{"imageloader", "imageloaderd", "imageloaderd", mntNS | restrictCaps | noNewPrivs | seccomp},
		&procReqs{"arc-networkd", "root", "root", noNewPrivs},
		&procReqs{"arc-networkd", "arc-networkd", "arc-networkd", restrictCaps},

		// These processes run as root in the ARC container.
		&procReqs{"app_process", "android-root", "android-root", pidNS | mntNS},
		&procReqs{"debuggerd", "android-root", "android-root", pidNS | mntNS},
		&procReqs{"debuggerd:sig", "android-root", "android-root", pidNS | mntNS},
		&procReqs{"healthd", "android-root", "android-root", pidNS | mntNS},
		&procReqs{"vold", "android-root", "android-root", pidNS | mntNS},

		// These processes run as non-root in the ARC container.
		&procReqs{"boot_latch", "656360", "656360", pidNS | mntNS | restrictCaps},
		&procReqs{"bugreportd", "657360", "656367", pidNS | mntNS | restrictCaps},
		&procReqs{"logd", "656396", "656396", pidNS | mntNS | restrictCaps},
		&procReqs{"servicemanager", "656360", "656360", pidNS | mntNS | restrictCaps},
		&procReqs{"surfaceflinger", "656360", "656363", pidNS | mntNS | restrictCaps},

		// Small, one-off init/setup scripts that don't spawn daemons and that are short-lived.
		&procReqs{"activate_date.service", "root", "root", 0},
		&procReqs{"crx-import.sh", "root", "root", 0},
		&procReqs{"lockbox-cache.sh", "root", "root", 0},
		&procReqs{"powerd-pre-start.sh", "root", "root", 0},
	}

	// exclusions contains names (from the "Name:" field in /proc/<pid>/status) of processes to ignore.
	exclusions := []string{
		"agetty",
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
		"x11vnc",
		"bash", // TODO: check against script name instead
		"dash",
		"python",
		"python2",
		"python2.7",
		"python3",
		"python3.4",
		"python3.5",
		"python3.6",
		"python3.7",
		"sh",
		"minijail0", // just launches other daemons; also runs as root to drop privs
		"minijail-init",
		"(agetty)", // initial name when systemd starts serial-getty; changes to "agetty" later
		"adb",      // sometimes appears on test images: https://crbug.com/792541
	}

	// Per TASK_COMM_LEN, the kernel only uses 16 null-terminated bytes to hold process names
	// (which we later read from /proc/<pid>/status), so we shorten names in the baseline and exclusion list.
	// See https://stackoverflow.com/questions/23534263 for more discussion.
	// TODO(derat): Find a better way of uniquely identifying processes. Using "Name:" from /status
	// matches what the Autotest test was doing, but it can lead to unexpected collisions. /exe is undesirable
	// since executables like /usr/bin/coreutils implement many commands. /cmdline may be modified by the process.
	const maxProcNameLen = 15
	truncateProcName := func(s string) string {
		if len(s) <= maxProcNameLen {
			return s
		}
		return s[:maxProcNameLen]
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

	asanEnabled, err := asan.Enabled(ctx)
	if err != nil {
		s.Error("Failed to check if ASan is enabled: ", err)
	} else if asanEnabled {
		s.Log("ASan is enabled; skipping seccomp checks")
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

	// We don't check the init process, but we'll use its info info later to determine
	// if other processes have their own capabilities/namespaces or not.
	var initInfo *procSandboxInfo

	// kthreadd is the parent of kernel processes, which we skip.
	var kthreaddPID int32

	// We also skip the Tast test runner process and its children.
	var tastTestRunnerPID int32
	tastTestRunnerName := truncateProcName("local_test_runner")

	// We don't know that we'll see parent processes before their children (since PIDs can wrap around),
	// so do an initial pass to gather information and identify the parents that we care about.
	infos := make(map[int32]*procSandboxInfo)
	for _, proc := range procs {
		info, err := getProcSandboxInfo(proc)
		if err != nil {
			// An error could either indicate that the process exited or that we failed to parse /proc.
			// Check if the process is still there so we can report the error in the latter case.
			// We ignore zombie processes since they seem to have missing namespace data.
			if status, serr := proc.Status(); serr == nil && status != "Z" {
				s.Errorf("Failed to get info about process %d: %v", proc.Pid, err)
			}
			continue
		}

		fmt.Fprintf(lg, "%5d %-15s uid=%-6d gid=%-6d pidns=%-10d mntns=%-10d nnp=%-5v seccomp=%-5v ecaps=%#x\n",
			proc.Pid, info.name, info.euid, info.egid, info.pidNS, info.mntNS, info.noNewPrivs, info.seccomp, info.ecaps)

		if proc.Pid == 1 {
			initInfo = info
		} else if info.name == "kthreadd" {
			kthreaddPID = proc.Pid
		} else if info.name == tastTestRunnerName {
			tastTestRunnerPID = proc.Pid
		} else {
			infos[proc.Pid] = info
		}
	}

	if initInfo == nil {
		s.Fatal("Didn't find init process")
	}
	if kthreaddPID == 0 {
		s.Fatal("Didn't find kthreadd process")
	}

	s.Logf("Comparing %d processes against baseline", len(infos))
	for pid, info := range infos {
		if info.ppid == kthreaddPID {
			continue
		}
		if _, ok := exclusionsMap[info.name]; ok {
			continue
		}
		if tastTestRunnerPID != 0 {
			if isTastProc, err := procHasAncestor(pid, tastTestRunnerPID); err == nil && isTastProc {
				continue
			}
		}

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

		hasPIDNS := info.pidNS != initInfo.pidNS
		hasMntNS := info.mntNS != initInfo.mntNS
		hasCaps := info.ecaps != initInfo.ecaps

		for _, st := range []struct {
			ft  feature // feature to check (not necessarily expected to be enabled)
			val bool    // whether feature is enabled or not for process
			msg string  // error message if feature is not present
		}{
			{pidNS, hasPIDNS, "missing PID namespace"},
			{mntNS, hasMntNS, "missing mount namespace"},
			{restrictCaps, hasCaps, "no restricted capabilities"},
			{noNewPrivs, info.noNewPrivs, "missing no_new_privs"},
			{seccomp, info.seccomp, "seccomp filter disabled"},
		} {
			// Minijail disables seccomp at runtime when ASan is enabled, so don't check it.
			if st.ft == seccomp && asanEnabled {
				continue
			}
			if reqs.features&st.ft != 0 && !st.val {
				problems = append(problems, st.msg)
			}
		}

		// If a mount namespace is used but some of the init process's test image mounts
		// are still present, then the process didn't call pivot_root().
		if hasMntNS && info.hasTestImageMounts {
			problems = append(problems, "did not call pivot_root(2)")
		}

		if len(problems) > 0 {
			s.Errorf("%q process %v (%v) isn't properly sandboxed: %s",
				info.name, pid, info.exe, strings.Join(problems, ", "))
		}
	}
}

// procSandboxInfo holds sandboxing-related information about a running process.
type procSandboxInfo struct {
	name               string // "Name:" value from /proc/<pid>/status
	exe                string // full executable path
	ppid               int32  // parent PID
	euid, egid         uint32 // effective UID and GID
	pidNS, mntNS       int64  // PID and mount namespace IDs
	ecaps              uint64 // effective capabilities
	noNewPrivs         bool   // no_new_privs is set (see "minijail -N")
	seccomp            bool   // seccomp filter is active
	hasTestImageMounts bool   // has test-image-only mounts
}

// getProcSandboxInfo returns sandboxing-related information about proc.
// An error is returned if any files cannot be read or if malformed data is encountered.
func getProcSandboxInfo(proc *process.Process) (*procSandboxInfo, error) {
	var info procSandboxInfo
	var err error

	info.exe, _ = proc.Exe() // ignore errors for e.g. kernel processes

	if info.ppid, err = proc.Ppid(); err != nil {
		return nil, errors.Wrap(err, "failed to get parent")
	}

	uids, err := proc.Uids()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get UIDs")
	}
	info.euid = uint32(uids[1])

	gids, err := proc.Gids()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GIDs")
	}
	info.egid = uint32(gids[1])

	if info.pidNS, err = readProcNamespace(proc.Pid, "pid"); err != nil {
		return nil, errors.Wrap(err, "failed to read pid namespace")
	}
	if info.mntNS, err = readProcNamespace(proc.Pid, "mnt"); err != nil {
		return nil, errors.Wrap(err, "failed to read mnt namespace")
	}

	// Read additional info from /proc/<pid>/status.
	status, err := readProcStatus(proc.Pid)
	if err != nil {
		return nil, errors.Wrap(err, "failed reading status")
	}
	if info.ecaps, err = strconv.ParseUint(status["CapEff"], 16, 64); err != nil {
		return nil, errors.Wrapf(err, "failed parsing effective caps %q", status["CapEff"])
	}
	info.name = status["Name"]
	info.noNewPrivs = status["NoNewPrivs"] == "1"
	info.seccomp = status["Seccomp"] == "2" // 1 is strict, 2 is filter

	// Check whether any mounts that only occur in test images are available to the process.
	// These are limited to the init mount namespace, so if a process has its own namespace,
	// it shouldn't have these.
	mnts, err := readProcMountpoints(proc.Pid)
	if err != nil {
		return nil, errors.Wrap(err, "failed reading mountpoints")
	}
	for _, mnt := range mnts {
		for _, tm := range []string{"/usr/local", "/var/db/pkg", "/var/lib/portage"} {
			if mnt == tm {
				info.hasTestImageMounts = true
				break
			}
		}
	}

	return &info, nil
}

// readProcNamespace returns pid's namespace ID for name (e.g. "pid" or "mnt"),
// per /proc/<pid>/ns/<name>.
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
func readProcMountpoints(pid int32) ([]string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/mounts", pid))
	if err != nil {
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

// procHasAncestor returns true if pid has ancestorPID as an ancestor process.
func procHasAncestor(pid, ancestorPID int32) (bool, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return false, err
	}
	for {
		pproc, err := proc.Parent()
		if err != nil {
			return false, err
		}
		if pproc.Pid == ancestorPID {
			return true, nil
		}
		if pproc.Pid == 1 {
			return false, nil
		}
		proc = pproc
	}
}
