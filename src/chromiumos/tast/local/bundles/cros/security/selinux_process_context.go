// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

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
			if proc.SEContext != context {
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
		exe searchType = iota
	)
	for _, testCase := range []struct {
		field   searchType
		query   string
		context string
	}{
		{exe, "/sbin/debugd", "u:r:cros_debugd:s0"},
		{exe, "/sbin/init", "u:r:cros_init:s0"},
		{exe, "/sbin/session_manager", "u:r:cros_session_manager:s0"},
		{exe, "/sbin/udevd", "u:r:cros_udevd:s0"},
		{exe, "/sbin/upstart-socket-bridge", "u:r:cros_upstart_socket_bridge:s0"},
		{exe, "/usr/bin/anomaly_collector", "u:r:cros_anomaly_collector:s0"},
		{exe, "/usr/bin/dbus-daemon", "u:r:cros_dbus_daemon:s0"},
		{exe, "/usr/bin/memd", "u:r:cros_memd:s0"},
		{exe, "/usr/bin/metrics_daemon", "u:r:cros_metrics_daemon:s0"},
		{exe, "/usr/bin/powerd", "u:r:cros_powerd:s0"},
		{exe, "/usr/sbin/ModemManager", "u:r:cros_modem_manager:s0"},
		{exe, "/usr/sbin/avahi-daemon", "u:r:cros_avahi_daemon:s0"},
		{exe, "/usr/sbin/chapsd", "u:r:cros_chapsd:s0"},
		{exe, "/usr/sbin/conntrackd", "u:r:cros_conntrackd:s0"},
		{exe, "/usr/sbin/cryptohomed", "u:r:cros_cryptohomed:s0"},
		{exe, "/usr/sbin/rsyslogd", "u:r:cros_rsyslogd:s0"},
		{exe, "/usr/sbin/wpa_supplicant", "u:r:wpa_supplicant:s0"},
		{exe, "/usr/sbin/update_engine", "u:r:cros_update_engine:s0"},
		{exe, "/usr/bin/tlsdated", "u:r:cros_tlsdated:s0"},
		{exe, "/usr/bin/cras", "u:r:cros_cras:s0"},
	} {
		var p []selinux.Process
		switch testCase.field {
		case exe:
			p = selinux.FindProcessesByExe(ps, testCase.query)
			if len(p) == 0 {
				s.Errorf("Cannot find processes for exe %q", testCase.query)
			}
		default:
			s.Errorf("%+v has invalid searchType %d", testCase, int(testCase.field))
			continue
		}
		assertContext(p, testCase.context)
	}
}
