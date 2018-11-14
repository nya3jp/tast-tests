// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxProcessContext,
		Desc:         "Checks that processes are running in correct SELinux domain",
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxProcessContext(ctx context.Context, s *testing.State) {
	assertContext := func(processes []selinux.Process, context string) {
		for _, proc := range processes {
			matched, err := regexp.MatchString(context, proc.SEContext)
			if err != nil {
				s.Errorf("Failed to check context for +%v; want %v", proc, context)
				continue
			}
			if !matched {
				s.Errorf("Process %+v has context %v; want %v", proc, proc.SEContext, context)
			}
		}
	}

	ps, err := selinux.GetProcesses()
	if err != nil {
		s.Fatal("Failed to get processes: ", err)
	}

	type searchType int
	const (
		exe     searchType = iota // absolute executable path
		cmdline                   // partial regular expression matched against command line
	)
	const (
		zeroProcs int = 0
		oneProc       = 1
		twoProcs      = 2
	)
	for _, testCase := range []struct {
		field   searchType
		query   string
		context string
		// Nonzero process counts should only be used for core services that are guaranteed to always be running.
		// Other tests that run before this one may restart non-critical daemons, so this test can't assume that
		// the processes will be there. The platform.CheckProcesses test is responsible for checking that processes
		// are actually running.
		// TODO(derat): Consider using oneProc again after updating this test to wait for services: https://crbug.com/897521
		minProcessCount int
	}{
		{cmdline, "/usr/bin/periodic_scheduler", "cros_periodic_scheduler", twoProcs},
		{exe, "/opt/google/chrome/chrome", "cros_browser", zeroProcs}, // Only when browser exists
		{exe, "/sbin/debugd", "cros_debugd", zeroProcs},
		{exe, "/sbin/init", "cros_init", oneProc},
		{exe, "/sbin/session_manager", "cros_session_manager", zeroProcs},
		{exe, "/sbin/udevd", "cros_udevd", oneProc},
		{exe, "/sbin/upstart-socket-bridge", "cros_upstart_socket_bridge", oneProc},
		{exe, "/usr/bin/anomaly_collector", "cros_anomaly_collector", zeroProcs},
		{exe, "/usr/bin/cras", "cros_cras", zeroProcs},
		{exe, "/usr/bin/cros-disks", "cros_disks", oneProc},
		{exe, "/usr/bin/dbus-daemon", "cros_dbus_daemon", oneProc},
		{exe, "/usr/bin/memd", "cros_memd", zeroProcs},
		{exe, "/usr/bin/metrics_daemon", "cros_metrics_daemon", zeroProcs},
		{exe, "/usr/bin/midis", "cros_midis", zeroProcs}, // Only after start-arc-instance
		{exe, "/usr/bin/powerd", "cros_powerd", zeroProcs},
		{exe, "/usr/bin/shill", "cros_shill", zeroProcs},
		{exe, "/usr/bin/sslh", "cros_sslh", zeroProcs},
		{exe, "/usr/bin/tlsdated", "cros_tlsdated", oneProc},
		{exe, "/usr/libexec/bluetooth/bluetoothd", "cros_bluetoothd", zeroProcs},
		{exe, "/usr/sbin/ModemManager", "cros_modem_manager", zeroProcs},
		{exe, "/usr/sbin/avahi-daemon", "cros_avahi_daemon", zeroProcs},
		{exe, "/usr/sbin/chapsd", "cros_chapsd", zeroProcs},
		{exe, "/usr/sbin/conntrackd", "cros_conntrackd", zeroProcs},
		{exe, "/usr/sbin/cryptohomed", "cros_cryptohomed", zeroProcs},
		{exe, "/usr/sbin/rsyslogd", "cros_rsyslogd", oneProc},
		{exe, "/usr/sbin/update_engine", "cros_update_engine", zeroProcs},
		{exe, "/usr/sbin/wpa_supplicant", "wpa_supplicant", zeroProcs},
	} {
		var p []selinux.Process
		var err error
		switch testCase.field {
		case exe:
			p = selinux.FindProcessesByExe(ps, testCase.query)
		case cmdline:
			p, err = selinux.FindProcessesByCmdline(ps, testCase.query)
		default:
			err = errors.Errorf("%+v has invalid searchType %d", testCase, int(testCase.field))
		}
		if err != nil {
			s.Errorf("Failed to find processes: %v", err)
			continue
		}
		s.Logf("Processes for %v: %v", testCase.query, p)
		if len(p) < testCase.minProcessCount {
			s.Errorf("Found %d process(es) for %v; require at least %d",
				len(p), testCase.query, testCase.minProcessCount)
		}
		// Also checks the context even number of processes is not enough.
		assertContext(p, selinux.S0Process(testCase.context))
	}
}
