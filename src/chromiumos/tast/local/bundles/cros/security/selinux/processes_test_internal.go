// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ProcessTestCaseSelector specifies what kind of test cases will be run.
type ProcessTestCaseSelector int

const (
	// Stable to run test cases proven to be stable.
	Stable ProcessTestCaseSelector = iota
	// Unstable to run newly introduced test cases or flaky cases.
	Unstable
)

const domainIsolationErrorMessage = "every daemon must have its own domain. Please follow step 1~3 of https://chromium.googlesource.com/chromiumos/docs/+/master/selinux.md#Practice-in-Examples to create a permissive domain for your daemons."

// ProcessesTestInternal runs the test suite for SELinuxProcesses(Experimental|Informational)?
func ProcessesTestInternal(ctx context.Context, s *testing.State, testSelector []ProcessTestCaseSelector) {
	assertContext := func(processes []Process, expected *regexp.Regexp, errorMsg string) {
		for _, proc := range processes {
			if !expected.MatchString(proc.SEContext) {
				if errorMsg != "" {
					s.Errorf("Process %+v has context %q; want %q; %v", proc, proc.SEContext, expected, errorMsg)
				} else {
					s.Errorf("Process %+v has context %q; want %q", proc, proc.SEContext, expected)
				}
			}
		}
	}
	// noStr returns a regular expression to match all strings that's not equal to s.
	// Golang regexp doesn't support lookahead, so we can't simply implement not in regexp.
	// e.g. notStr("abc") => "(([^a].*)?|a([^b].*)?|ab([^c].*)?|abc.+)"
	notStr := func(s string) string {
		l := len(s)
		rt := make([]string, 0)
		for i := 0; i < l; i++ {
			rt = append(rt, s[0:i]+"([^"+s[i:i+1]+"].*)?")
		}
		rt = append(rt, s+".+")
		return "(" + strings.Join(rt, "|") + ")"
	}

	ps, err := GetProcesses()
	if err != nil {
		s.Fatal("Failed to get processes: ", err)
	}

	type searchType int
	const (
		exe        searchType = iota // absolute executable path
		notExe                       // not absolute executable path
		cmdline                      // partial regular expression matched against command line
		notCmdline                   // not matching the regular expression
	)
	const (
		zeroProcs int = 0
		oneProc       = 1
		twoProcs      = 2
	)
	type testCaseType struct {
		field   searchType // field to search for processes
		query   string     // search keyword for given field
		context string     // expected SELinux process context (domain).
		// Nonzero process counts should only be used for core services that are guaranteed to always be running.
		// Other tests that run before this one may restart non-critical daemons, so this test can't assume that
		// the processes will be there. The platform.CheckProcesses test is responsible for checking that processes
		// are actually running.
		minProcessCount int
		errorMsg        string // an optional error message that may help developers understand why it fails or how to fix.
	}

	testCases := make([]testCaseType, 0)
	for _, sel := range testSelector {
		switch sel {
		case Stable:
			testCases = append(testCases, []testCaseType{
				{cmdline, ".*logger.*-t arc-kmsg-logger.*", "cros_arc_kmsg_logger", zeroProcs, ""},
				{cmdline, ".*/usr/bin/periodic_scheduler.*", "cros_periodic_scheduler", twoProcs, ""},
				{cmdline, ".*/usr/share/cros/init/activate_date.sh.*", "cros_activate_date", zeroProcs, ""},
				{cmdline, "/system/bin/sdcard.*", "cros_arc_sdcardd", zeroProcs, ""},
				{exe, "/opt/google/chrome/chrome", "cros_browser", zeroProcs, ""}, // Only when browser exists
				{exe, "/sbin/auditd", "cros_auditd", oneProc, ""},                 // auditd must be running on SELinux boards
				{exe, "/sbin/debugd", "cros_debugd", zeroProcs, ""},
				{exe, "/sbin/init", "cros_init", oneProc, ""},
				{exe, "/sbin/minijail0", "(minijail|.*_minijail0)", zeroProcs, ""},
				{exe, "/sbin/session_manager", "cros_session_manager", zeroProcs, ""},
				{exe, "/sbin/udevd", "cros_udevd", oneProc, ""},
				{exe, "/sbin/upstart-socket-bridge", "cros_upstart_socket_bridge", oneProc, ""},
				{exe, "/usr/bin/anomaly_detector", "cros_anomaly_detector", zeroProcs, ""},
				{exe, "/usr/bin/arc-appfuse-provider", "cros_arc_appfuse_provider", zeroProcs, ""},
				{exe, "/usr/bin/arc-networkd", "cros_arc_networkd", zeroProcs, ""},
				{exe, "/usr/bin/arc-obb-mounter", "cros_arc_obb_mounter", zeroProcs, ""},
				{exe, "/usr/bin/arc_camera_service", "cros_arc_camera_service", zeroProcs, ""},
				{exe, "/usr/bin/biod", "cros_biod", zeroProcs, ""},
				{exe, "/usr/bin/btdispatch", "cros_btdispatch", zeroProcs, ""},
				{exe, "/usr/bin/cras", "cros_cras", zeroProcs, ""},
				{exe, "/usr/bin/cros-disks", "cros_disks", oneProc, ""},
				{exe, "/usr/bin/cros_camera_algo", "cros_camera_algo", zeroProcs, ""},
				{exe, "/usr/bin/cros_camera_service", "cros_camera_service", zeroProcs, ""},
				{exe, "/usr/bin/dbus-daemon", "cros_dbus_daemon", oneProc, ""},
				{exe, "/usr/bin/esif_ufd", "cros_esif_ufd", zeroProcs, ""},
				{exe, "/usr/bin/memd", "cros_memd", zeroProcs, ""},
				{exe, "/usr/bin/metrics_daemon", "cros_metrics_daemon", zeroProcs, ""},
				{exe, "/usr/bin/midis", "cros_midis", zeroProcs, ""}, // Only after start-arc-instance
				{exe, "/usr/bin/ml_service", "cros_ml_service", zeroProcs, ""},
				{exe, "/usr/bin/modemfwd", "cros_modemfwd", zeroProcs, ""},
				{exe, "/usr/bin/mount-passthrough", "cros_mount_passthrough", zeroProcs, ""},
				{exe, "/usr/bin/newblued", "cros_newblued", zeroProcs, ""},
				{exe, "/usr/bin/permission_broker", "cros_permission_broker", zeroProcs, ""},
				{exe, "/usr/bin/powerd", "cros_powerd", zeroProcs, ""},
				{exe, "/usr/bin/run_oci", "cros_arc_setup", zeroProcs, ""},
				{exe, "/usr/bin/shill", "cros_shill", zeroProcs, ""},
				{exe, "/usr/bin/sslh", "cros_sslh", zeroProcs, ""},
				{exe, "/usr/bin/timberslide", "cros_timberslide", zeroProcs, ""},
				{exe, "/usr/bin/tlsdated", "cros_tlsdated", zeroProcs, ""},
				{exe, "/usr/bin/u2fd", "cros_u2fd", zeroProcs, ""},
				{exe, "/usr/bin/vmlog_forwarder", "cros_vmlog_forwarder", zeroProcs, ""},
				{exe, "/usr/lib/systemd/systemd-journald", "cros_journald", zeroProcs, ""},
				{exe, "/usr/libexec/bluetooth/bluetoothd", "cros_bluetoothd", zeroProcs, ""},
				{exe, "/usr/sbin/ModemManager", "cros_modem_manager", zeroProcs, ""},
				{exe, "/usr/sbin/arc-keymasterd", "cros_arc_keymasterd", zeroProcs, ""},
				{exe, "/usr/sbin/arc-oemcrypto", "cros_arc_oemcrypto", zeroProcs, ""},
				{exe, "/usr/sbin/arc-setup", "cros_arc_setup", zeroProcs, ""},
				{exe, "/usr/sbin/atrusd", "cros_atrusd", zeroProcs, ""},
				{exe, "/usr/sbin/attestationd", "cros_attestationd", zeroProcs, ""},
				{exe, "/usr/sbin/avahi-daemon", "cros_avahi_daemon", zeroProcs, ""},
				{exe, "/usr/sbin/bootlockboxd", "cros_bootlockboxd", zeroProcs, ""},
				{exe, "/usr/sbin/cecservice", "cros_cecservice", zeroProcs, ""},
				{exe, "/usr/sbin/chapsd", "cros_chapsd", zeroProcs, ""},
				{exe, "/usr/sbin/conntrackd", "cros_conntrackd", zeroProcs, ""},
				{exe, "/usr/sbin/cryptohomed", "cros_cryptohomed", zeroProcs, ""},
				{exe, "/usr/sbin/daisydog", "cros_daisydog", zeroProcs, ""},
				{exe, "/usr/sbin/dlcservice", "cros_dlcservice", zeroProcs, ""},
				{exe, "/usr/sbin/huddly-monitor", "cros_huddly_monitor", zeroProcs, ""},
				{exe, "/usr/sbin/mimo-minitor", "cros_mimo_monitor", zeroProcs, ""},
				{exe, "/usr/sbin/mtpd", "cros_mtpd", zeroProcs, ""},
				{exe, "/usr/sbin/oobe_config_restore", "cros_oobe_config_restore", zeroProcs, ""},
				{exe, "/usr/sbin/rsyslogd", "cros_rsyslogd", oneProc, ""},
				{exe, "/usr/sbin/sshd", "cros_sshd", zeroProcs, ""},
				{exe, "/usr/sbin/tcsd", "cros_tcsd", zeroProcs, ""},
				{exe, "/usr/sbin/tpm_managerd", "cros_tpm_managerd", zeroProcs, ""},
				{exe, "/usr/sbin/trunksd", "cros_trunksd", zeroProcs, ""},
				{exe, "/usr/sbin/update_engine", "cros_update_engine", zeroProcs, ""},
				{exe, "/usr/sbin/wpa_supplicant", "wpa_supplicant", zeroProcs, ""},
				{notCmdline, ".*(frecon|agetty|ping|recover_duts).*", notStr("chromeos"), zeroProcs, domainIsolationErrorMessage},
				{notCmdline, ".*(frecon|agetty|ping|recover_duts).*", notStr("minijailed"), zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/init", notStr("cros_init"), zeroProcs, domainIsolationErrorMessage},
				// coreutils and ping are excluded for recover_duts scripts.
				{notExe, "(/bin/([db]a)?sh|/usr/bin/coreutils|/bin/ping)", notStr("cros_init_scripts"), zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/minijail0", notStr("minijail"), zeroProcs, domainIsolationErrorMessage},
			}...)
		case Unstable:
			testCases = append(testCases, []testCaseType{
				{exe, "/sbin/minijail0", "(minijail|.*_minijail0)", zeroProcs, ""},
				{notExe, "/bin/([db]a)?sh", notStr("cros_init_scripts"), zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/init", notStr("cros_init"), zeroProcs, domainIsolationErrorMessage},
				// Continue monitoring frecon/agetty/recover_duts[ping].
				{cmdline, ".*", notStr("chromeos"), zeroProcs, domainIsolationErrorMessage},
				{cmdline, ".*", notStr("minijailed"), zeroProcs, domainIsolationErrorMessage},
			}...)
		}
	}

	for _, testCase := range testCases {
		var p []Process
		var err error
		switch testCase.field {
		case exe:
			p, err = FindProcessesByExe(ps, testCase.query, false)
		case notExe:
			p, err = FindProcessesByExe(ps, testCase.query, true)
		case cmdline:
			p, err = FindProcessesByCmdline(ps, testCase.query, false)
		case notCmdline:
			p, err = FindProcessesByCmdline(ps, testCase.query, true)
		default:
			err = errors.Errorf("%+v has invalid searchType %d", testCase, int(testCase.field))
		}
		if err != nil {
			s.Error("Failed to find processes: ", err)
			continue
		}
		s.Logf("Processes for %v: %v", testCase.query, p)
		if len(p) < testCase.minProcessCount {
			s.Errorf("Found %d process(es) for %v; require at least %d",
				len(p), testCase.query, testCase.minProcessCount)
		}
		// Also checks the context even number of processes is not enough.
		expected, err := ProcessContextRegexp(testCase.context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testCase.context, err)
			continue
		}
		assertContext(p, expected, testCase.errorMsg)
	}
}
