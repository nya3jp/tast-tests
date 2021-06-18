// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"context"
	"fmt"
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

const domainIsolationErrorMessage = "every daemon must have its own domain. Please follow steps 1~3 of https://chromium.googlesource.com/chromiumos/docs/+/main/security/selinux.md#Practice-in-Examples to create a permissive domain for your daemon."

// ProcessesTestInternal runs the test suite for SELinuxProcesses(Experimental|Informational)?
func ProcessesTestInternal(ctx context.Context, s *testing.State, testSelector []ProcessTestCaseSelector) {
	type processSearchType int
	const (
		exe        processSearchType = iota // absolute executable path
		notExe                              // not absolute executable path
		cmdline                             // partial regular expression matched against command line
		notCmdline                          // not matching the regular expression
	)
	type contextMatchType int
	const (
		matchRegexp contextMatchType = iota // matches a regexp positively
		notString                           // does not include string
	)
	const (
		zeroProcs int = 0
		oneProc       = 1
		twoProcs      = 2
	)
	type testCaseType struct {
		field        processSearchType // field to search for processes
		query        string            // search keyword for given field
		contextMatch contextMatchType  // how to match the expected SELinux context (domain)
		context      string            // expected SELinux process context (domain).
		// Nonzero process counts should only be used for core services that are guaranteed to always be running.
		// Other tests that run before this one may restart non-critical daemons, so this test can't assume that
		// the processes will be there. The platform.CheckProcesses test is responsible for checking that processes
		// are actually running.
		minProcessCount int
		errorMsg        string // an optional error message that may help developers understand why it fails or how to fix.
	}

	assertContext := func(processes []Process, testCase testCaseType) {
		for _, proc := range processes {
			var errorLine strings.Builder
			fmt.Fprintf(&errorLine, "Process %+v has context %q", proc, proc.SEContext)

			switch testCase.contextMatch {
			case matchRegexp:
				expectedContext, err := ProcessContextRegexp(testCase.context)
				if err != nil {
					s.Errorf("Failed to compile expected context %q: %v", testCase.context, err)
					return
				}
				if !expectedContext.MatchString(proc.SEContext) {
					fmt.Fprintf(&errorLine, "; want %q", expectedContext)
					if testCase.errorMsg != "" {
						fmt.Fprintf(&errorLine, "; %v", testCase.errorMsg)
					}
					s.Error(errorLine.String())
				}
			case notString:
				if strings.Contains(proc.SEContext, ":"+testCase.context+":") {
					fmt.Fprintf(&errorLine, "; want anything except %q", testCase.context)
					if testCase.errorMsg != "" {
						fmt.Fprintf(&errorLine, "; %v", testCase.errorMsg)
					}
					s.Error(errorLine.String())
				}
			default:
				s.Errorf("%+v has invalid contextMatchType %d", testCase, int(testCase.contextMatch))
			}
		}
	}

	ps, err := GetProcesses()
	if err != nil {
		s.Fatal("Failed to get processes: ", err)
	}

	testCases := make([]testCaseType, 0)
	for _, sel := range testSelector {
		switch sel {
		case Stable:
			testCases = append(testCases, []testCaseType{
				{cmdline, ".*logger.*-t arc-kmsg-logger.*", matchRegexp, "cros_arc_kmsg_logger", zeroProcs, ""},
				{cmdline, ".*/usr/share/cros/init/activate_date.sh.*", matchRegexp, "cros_activate_date", zeroProcs, ""},
				{cmdline, "/system/bin/sdcard.*", matchRegexp, "cros_arc_sdcardd", zeroProcs, ""},
				{exe, "/opt/google/chrome/chrome", matchRegexp, "cros_browser", zeroProcs, ""}, // Only when browser exists
				{exe, "/opt/google/easy_unlock/easy_unlock", matchRegexp, "cros_easy_unlock", zeroProcs, ""},
				{exe, "/sbin/auditd", matchRegexp, "cros_auditd", oneProc, ""}, // auditd must be running on SELinux boards
				{exe, "/sbin/debugd", matchRegexp, "cros_debugd", zeroProcs, ""},
				{exe, "/sbin/init", matchRegexp, "cros_init", oneProc, ""},
				{exe, "/sbin/minijail0", matchRegexp, "(minijail|.*_minijail0|cros_.*_minijail)", zeroProcs, ""},
				{exe, "/sbin/session_manager", matchRegexp, "cros_session_manager", zeroProcs, ""},
				{exe, "/sbin/udevd", matchRegexp, "cros_udevd", oneProc, ""},
				{exe, "/sbin/upstart-socket-bridge", matchRegexp, "cros_upstart_socket_bridge", oneProc, ""},
				{exe, "/usr/bin/anomaly_detector", matchRegexp, "cros_anomaly_detector", zeroProcs, ""},
				{exe, "/usr/bin/arc-appfuse-provider", matchRegexp, "cros_arc_appfuse_provider", zeroProcs, ""},
				{exe, "/usr/bin/arc-obb-mounter", matchRegexp, "cros_arc_obb_mounter", zeroProcs, ""},
				{exe, "/usr/bin/arc_camera_service", matchRegexp, "cros_arc_camera_service", zeroProcs, ""},
				{exe, "/usr/bin/biod", matchRegexp, "cros_biod", zeroProcs, ""},
				{exe, "/usr/bin/btdispatch", matchRegexp, "cros_btdispatch", zeroProcs, ""},
				{exe, "/usr/bin/chunneld", matchRegexp, "cros_chunneld", zeroProcs, ""},
				{exe, "/usr/bin/cras", matchRegexp, "cros_cras", zeroProcs, ""},
				{exe, "/usr/bin/cros-disks", matchRegexp, "cros_disks", oneProc, ""},
				{exe, "/usr/bin/cros_camera_algo", matchRegexp, "cros_camera_algo", zeroProcs, ""},
				{exe, "/usr/bin/cros_camera_service", matchRegexp, "cros_camera_service", zeroProcs, ""},
				{exe, "/usr/bin/cups_proxy", matchRegexp, "cros_cups_proxy", zeroProcs, ""},
				{exe, "/usr/bin/dbus-daemon", matchRegexp, "cros_dbus_daemon", oneProc, ""},
				{exe, "/usr/bin/esif_ufd", matchRegexp, "cros_esif_ufd", zeroProcs, ""},
				{exe, "/usr/bin/memd", matchRegexp, "cros_memd", zeroProcs, ""},
				{exe, "/usr/bin/metrics_daemon", matchRegexp, "cros_metrics_daemon", zeroProcs, ""},
				{exe, "/usr/bin/midis", matchRegexp, "cros_midis", zeroProcs, ""}, // Only after start-arc-instance
				{exe, "/usr/bin/ml_service", matchRegexp, "cros_ml_service", zeroProcs, ""},
				{exe, "/usr/bin/modemfwd", matchRegexp, "cros_modemfwd", zeroProcs, ""},
				{exe, "/usr/bin/mount-passthrough", matchRegexp, "cros_mount_passthrough", zeroProcs, ""},
				{exe, "/usr/bin/mount-passthrough-jailed", matchRegexp, "cros_mount_passthrough_jailed", zeroProcs, ""},
				{exe, "/usr/bin/mount-passthrough-jailed-media", matchRegexp, "cros_mount_passthrough_jailed_media", zeroProcs, ""},
				{exe, "/usr/bin/mount-passthrough-jailed-play", matchRegexp, "cros_mount_passthrough_jailed_play", zeroProcs, ""},
				{exe, "/usr/bin/newblued", matchRegexp, "cros_newblued", zeroProcs, ""},
				{exe, "/usr/bin/patchpaneld", matchRegexp, "cros_patchpaneld", zeroProcs, ""},
				{exe, "/usr/bin/periodic_scheduler", matchRegexp, "cros_periodic_scheduler", twoProcs, ""},
				{exe, "/usr/bin/permission_broker", matchRegexp, "cros_permission_broker", zeroProcs, ""},
				{exe, "/usr/bin/powerd", matchRegexp, "cros_powerd", zeroProcs, ""},
				{exe, "/usr/bin/resourced", matchRegexp, "cros_resourced", zeroProcs, ""},
				{exe, "/usr/bin/run_oci", matchRegexp, "cros_arc_setup", zeroProcs, ""},
				{exe, "/usr/bin/seneschal", matchRegexp, "cros_seneschal", zeroProcs, ""},
				{exe, "/usr/bin/shill", matchRegexp, "cros_shill", zeroProcs, ""},
				{exe, "/usr/bin/sslh", matchRegexp, "cros_sslh", zeroProcs, ""},
				{exe, "/usr/bin/timberslide", matchRegexp, "cros_timberslide", zeroProcs, ""},
				{exe, "/usr/bin/tlsdated", matchRegexp, "cros_tlsdated", zeroProcs, ""},
				{exe, "/usr/bin/tpm2-simulator", matchRegexp, "cros_tpm2_simulator", zeroProcs, ""},
				{exe, "/usr/bin/traced", matchRegexp, "cros_traced", zeroProcs, ""},
				{exe, "/usr/bin/traced_probes", matchRegexp, "cros_traced_probes", zeroProcs, ""},
				{exe, "/usr/bin/u2fd", matchRegexp, "cros_u2fd", zeroProcs, ""},
				{exe, "/usr/bin/virtual-file-provider", matchRegexp, "cros_virtual_file_provider", zeroProcs, ""},
				{exe, "/usr/bin/virtual-file-provider-jailed", matchRegexp, "cros_virtual_file_provider_jailed", zeroProcs, ""},
				{exe, "/usr/bin/vm_cicerone", matchRegexp, "cros_vm_cicerone", zeroProcs, ""},
				{exe, "/usr/bin/vm_concierge", matchRegexp, "cros_vm_concierge", zeroProcs, ""},
				{exe, "/usr/bin/vmlog_forwarder", matchRegexp, "cros_vmlog_forwarder", zeroProcs, ""},
				{exe, "/usr/lib/systemd/systemd-journald", matchRegexp, "cros_journald", zeroProcs, ""},
				{exe, "/usr/libexec/bluetooth/bluetoothd", matchRegexp, "cros_bluetoothd", zeroProcs, ""},
				{exe, "/usr/sbin/ModemManager", matchRegexp, "cros_modem_manager", zeroProcs, ""},
				{exe, "/usr/sbin/arc-keymasterd", matchRegexp, "cros_arc_keymasterd", zeroProcs, ""},
				{exe, "/usr/sbin/arc-setup", matchRegexp, "cros_arc_setup", zeroProcs, ""},
				{exe, "/usr/sbin/atrusd", matchRegexp, "cros_atrusd", zeroProcs, ""},
				{exe, "/usr/sbin/attestationd", matchRegexp, "cros_attestationd", zeroProcs, ""},
				{exe, "/usr/sbin/avahi-daemon", matchRegexp, "cros_avahi_daemon", zeroProcs, ""},
				{exe, "/usr/sbin/bootlockboxd", matchRegexp, "cros_bootlockboxd", zeroProcs, ""},
				{exe, "/usr/sbin/cdm-oemcrypto", matchRegexp, "cros_cdm_oemcrypto", zeroProcs, ""},
				{exe, "/usr/sbin/cecservice", matchRegexp, "cros_cecservice", zeroProcs, ""},
				{exe, "/usr/sbin/chapsd", matchRegexp, "cros_chapsd", zeroProcs, ""},
				{exe, "/usr/sbin/conntrackd", matchRegexp, "cros_conntrackd", zeroProcs, ""},
				{exe, "/usr/sbin/crosdns", matchRegexp, "cros_crosdns", zeroProcs, ""},
				{exe, "/usr/sbin/cryptohomed", matchRegexp, "cros_cryptohomed", zeroProcs, ""},
				{exe, "/usr/sbin/cryptohome-proxy", matchRegexp, "cros_cryptohome_proxy", zeroProcs, ""},
				{exe, "/usr/sbin/daisydog", matchRegexp, "cros_daisydog", zeroProcs, ""},
				{exe, "/usr/sbin/dlcservice", matchRegexp, "cros_dlcservice", zeroProcs, ""},
				{exe, "/usr/sbin/huddly-monitor", matchRegexp, "cros_huddly_monitor", zeroProcs, ""},
				{exe, "/usr/sbin/mimo-minitor", matchRegexp, "cros_mimo_monitor", zeroProcs, ""},
				{exe, "/usr/sbin/mtpd", matchRegexp, "cros_mtpd", zeroProcs, ""},
				{exe, "/usr/sbin/oobe_config_restore", matchRegexp, "cros_oobe_config_restore", zeroProcs, ""},
				{exe, "/usr/sbin/rsyslogd", matchRegexp, "cros_rsyslogd", oneProc, ""},
				{exe, "/usr/sbin/sshd", matchRegexp, "cros_sshd", zeroProcs, ""},
				{exe, "/usr/sbin/tcsd", matchRegexp, "cros_tcsd", zeroProcs, ""},
				{exe, "/usr/sbin/tpm_managerd", matchRegexp, "cros_tpm_managerd", zeroProcs, ""},
				{exe, "/usr/sbin/trunksd", matchRegexp, "cros_trunksd", zeroProcs, ""},
				{exe, "/usr/sbin/update_engine", matchRegexp, "cros_update_engine", zeroProcs, ""},
				{exe, "/usr/sbin/usbguard-daemon", matchRegexp, "cros_usbguard", zeroProcs, ""},
				{exe, "/usr/sbin/wpa_supplicant", matchRegexp, "wpa_supplicant", zeroProcs, ""},
				// moblab, autotest, devserver, rotatelogs, apache2, envoy, containerd are all required for
				// normal operation of moblab devices.
				// python3 is for crbug.com/1151463.
				// mkdir is for crbug.com/1156295.
				{notCmdline, ".*(frecon|agetty|ping|recover_dts|udevadm|update_rw_vpd|mosys|vpd|flashrom|moblab|autotest|devserver|rotatelogs|apache2|envoy|containerd|python3|mkdir).*", notString, "chromeos", zeroProcs, domainIsolationErrorMessage},
				{notCmdline, ".*(frecon|agetty|ping|recover_duts).*", notString, "minijailed", zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/init", notString, "cros_init", zeroProcs, domainIsolationErrorMessage},
				// coreutils and ping are excluded for recover_duts scripts.
				// logger is common to redirect output widely used from init conf scripts.
				{notExe, "(/bin/([db]a)?sh|/usr/bin/coreutils|/usr/bin/logger|/bin/ping|brcm_patchram_plus)", notString, "cros_init_scripts", zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/minijail0", notString, "minijail", zeroProcs, domainIsolationErrorMessage},
			}...)
		case Unstable:
			testCases = append(testCases, []testCaseType{
				{exe, "/sbin/minijail0", matchRegexp, "(minijail|.*_minijail0|cros_.*_minijail)", zeroProcs, ""},
				{notExe, "(/bin/([db]a)?sh|/usr/bin/coreutils|/usr/bin/logger)", notString, "cros_init_scripts", zeroProcs, domainIsolationErrorMessage},
				{notExe, "/sbin/init", notString, "cros_init", zeroProcs, domainIsolationErrorMessage},
				{notCmdline, ".*(ping|frecon|agetty|recover_duts).*", notString, "chromeos", zeroProcs, domainIsolationErrorMessage},
				{cmdline, ".*", notString, "minijailed", zeroProcs, domainIsolationErrorMessage},
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
			err = errors.Errorf("%+v has invalid processSearchType %d", testCase, int(testCase.field))
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
		// Also checks the context even if the number of processes is not enough.
		assertContext(p, testCase)
	}
}
