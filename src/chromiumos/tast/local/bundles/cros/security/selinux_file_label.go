// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"syscall"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFileLabel,
		Desc:         "Checks that SELinux file labels are set correctly",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFileLabel(ctx context.Context, s *testing.State) {
	systemCPUFilter := func(p string, fi os.FileInfo) (skipFile, skipSubdir selinux.FilterResult) {
		mode := fi.Mode()
		if mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0) {
			return selinux.Skip, selinux.Check
		}
		return selinux.Check, selinux.Check
	}

	// Files to be tested.
	// Files should have been labeled by platform2/sepolicy/file_contexts/ or
	// platform2/sepolicy/policy/*/genfs_contexts with a few exceptions.
	// Exceptions include:
	//  - type_transition rule to default assign a label for files created
	// under some condition.
	//  - mv/cp files without preserving original labels but inheriting
	// labels from new parent directory (e.g. /var/log/mount-encrypted.log)
	testArgs := []struct {
		path, context string
		recursive     bool
		filter        selinux.FileLabelCheckFilter
	}{
		{"/bin", "cros_coreutils_exec", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{"/bin/bash", "sh_exec", false, nil},
		{"/bin/dash", "sh_exec", false, nil},
		{"/bin/kmod", "cros_modprobe_exec", false, nil},
		{"/bin/sh", "sh_exec", false, nil},
		{"/etc/group", "cros_passwd_file", false, nil},
		{"/etc/hosts", "cros_network_conf_file", false, nil},
		{"/etc/hosts.d", "cros_network_conf_file", true, selinux.SkipNotExist},
		{"/etc/init", "cros_init_conf_file", true, nil},
		{"/etc/ld.so.cache", "cros_ld_conf_cache", false, nil},
		{"/etc/ld.so.conf", "cros_ld_conf_cache", false, nil},
		{"/etc/nsswitch.conf", "cros_network_conf_file", false, nil},
		{"/etc/passwd", "cros_passwd_file", false, nil},
		{"/etc/resolv.conf", "cros_network_conf_file", false, nil},
		{"/etc/rsyslog.chromeos", "cros_rsyslog_conf_file", false, nil},
		{"/etc/rsyslog.conf", "cros_rsyslog_conf_file", false, nil},
		{"/etc/rsyslog.d", "cros_rsyslog_conf_file", true, nil},
		{"/run/cras", "cras_socket", true, nil},
		{"/run/dbus", "cros_run_dbus", false, nil},
		{"/run/dbus.pid", "cros_dbus_pid_file", false, nil},
		{"/run/dbus/system_bus_socket", "cros_system_bus_socket", false, nil},
		{"/sbin/crash_reporter", "cros_crash_reporter_exec", false, nil},
		{"/sbin/crash_sender", "cros_crash_sender_exec", false, nil},
		{"/sbin/debugd", "cros_debugd_exec", false, nil},
		{"/sbin/frecon", "frecon_exec", false, nil},
		{"/sbin/init", "chromeos_init_exec", false, nil},
		{"/sbin/insmod", "cros_modprobe_exec", false, nil},
		{"/sbin/minijail0", "cros_minijail_exec", false, nil},
		{"/sbin/modprobe", "cros_modprobe_exec", false, nil},
		{"/sbin/rmmod", "cros_modprobe_exec", false, nil},
		{"/sbin/session_manager", "cros_session_manager_exec", false, nil},
		{"/sbin/udevd", "cros_udevd_exec", false, nil},
		{"/sbin/upstart-socket-bridge", "upstart_socket_bridge_exec", false, nil},
		{"/sys/devices/system/cpu", "sysfs", true, selinux.InvertFilterSkipFile(systemCPUFilter)},
		{"/sys/devices/system/cpu", "sysfs_devices_system_cpu", true, systemCPUFilter},
		{"/sys/fs/cgroup", "cgroup", true, selinux.IgnorePathButNotContents("/sys/fs/cgroup")},
		{"/sys/fs/cgroup", "tmpfs", false, nil},
		{"/sys/fs/pstore", "pstorefs", false, nil},
		{"/sys/fs/selinux", "selinuxfs", true, selinux.IgnorePathButNotContents("/sys/fs/selinux/null")},
		{"/sys/fs/selinux/null", "null_device", false, nil},
		{"/sys/kernel/config", "configfs", false, selinux.SkipNotExist},
		{"/sys/kernel/debug", "debugfs", false, nil},
		{"/sys/kernel/debug/debugfs_tracing_on", "debugfs_tracing", false, selinux.SkipNotExist},
		{"/sys/kernel/debug/tracing", "debugfs_tracing", false, nil},
		{"/sys/kernel/debug/tracing/trace_marker", "debugfs_trace_marker", false, selinux.SkipNotExist},
		{"/usr/bin", "cros_coreutils_exec", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{"/usr/bin/anomaly_collector", "cros_anomaly_collector_exec", false, nil},
		{"/usr/bin/chrt", "cros_chrt_exec", false, nil},
		{"/usr/bin/cras", "cros_cras_exec", false, nil},
		{"/usr/bin/dbus-daemon", "cros_dbus_daemon_exec", false, nil},
		{"/usr/bin/ionice", "cros_ionice_exec", false, nil},
		{"/usr/bin/logger", "cros_logger_exec", false, nil},
		{"/usr/bin/memd", "cros_memd_exec", false, nil},
		{"/usr/bin/metrics_daemon", "cros_metrics_daemon_exec", false, nil},
		{"/usr/bin/midis", "cros_midis_exec", false, selinux.SkipNotExist},
		{"/usr/bin/periodic_scheduler", "cros_periodic_scheduler_exec", false, nil},
		{"/usr/bin/powerd", "cros_powerd_exec", false, nil},
		{"/usr/bin/shill", "cros_shill_exec", false, nil},
		{"/usr/bin/start_bluetoothd.sh", "cros_init_start_bluetoothd_shell_script", false, nil},
		{"/usr/bin/tlsdated", "cros_tlsdated_exec", false, nil},
		{"/usr/sbin/ModemManager", "cros_modem_manager_exec", false, nil},
		{"/usr/sbin/avahi-daemon", "cros_avahi_daemon_exec", false, nil},
		{"/usr/sbin/chapsd", "cros_chapsd_exec", false, nil},
		{"/usr/sbin/chromeos-cleanup-logs", "cros_chromeos_cleanup_logs_exec", false, nil},
		{"/usr/sbin/chromeos-trim", "cros_chromeos_trim_exec", false, nil},
		{"/usr/sbin/conntrackd", "cros_conntrackd_exec", false, nil},
		{"/usr/sbin/cros-machine-id-regen", "cros_machine_id_regen_exec", false, nil},
		{"/usr/sbin/cryptohomed", "cros_cryptohomed_exec", false, nil},
		{"/usr/sbin/rsyslogd", "cros_rsyslogd_exec", false, nil},
		{"/usr/sbin/sslh", "cros_sslh_exec", false, selinux.SkipNotExist},
		{"/usr/sbin/sslh-fork", "cros_sslh_exec", false, selinux.SkipNotExist},
		{"/usr/sbin/sslh-select", "cros_sslh_exec", false, selinux.SkipNotExist},
		{"/usr/sbin/update_engine", "cros_update_engine_exec", false, nil},
		{"/usr/sbin/wpa_supplicant", "cros_wpa_supplicant_exec", false, nil},
		{"/usr/share/cros/init", "cros_init_shell_scripts", true, selinux.IgnorePathsButNotContents([]string{
			"/usr/share/cros/init/activate_date.sh",
			"/usr/share/cros/init/crx-import.sh",
			"/usr/share/cros/init/lockbox-cache.sh",
			"/usr/share/cros/init/powerd-pre-start.sh",
			"/usr/share/cros/init/ui-pre-start",
			"/usr/share/cros/init/ui-respawn",
		})},
		{"/usr/share/cros/init/activate_date.sh", "cros_init_activate_date_script", false, selinux.SkipNotExist},
		{"/usr/share/cros/init/crx-import.sh", "cros_init_crx_import_script", false, nil},
		{"/usr/share/cros/init/lockbox-cache.sh", "cros_init_lockbox_cache_script", false, nil},
		{"/usr/share/cros/init/powerd-pre-start.sh", "cros_init_powerd_pre_start_script", false, nil},
		{"/usr/share/cros/init/ui-pre-start", "cros_init_ui_pre_start_shell_script", false, nil},
		{"/usr/share/cros/init/ui-respawn", "cros_init_ui_respawn_shell_script", false, nil},
		{"/var", "cros_var", false, nil},
		{"/var/empty", "cros_var_empty", false, nil},
		{"/var/lib", "cros_var_lib", false, nil},
		{"/var/lib/dbus", "cros_var_lib_dbus", true, nil},
		{"/var/lib/metrics", "cros_metrics_file", true, selinux.IgnorePathButNotContents("/var/lib/metrics/uma-events")},
		{"/var/lib/metrics/uma-events", "cros_metrics_uma_events_file", false, nil},
		{"/var/log", "cros_var_log", false, nil},
		{"/var/log/arc.log", "cros_arc_log", false, nil},
		{"/var/log/authpolicy.log", "cros_authpolicy_log", false, nil},
		{"/var/log/boot.log", "cros_boot_log", false, nil},
		{"/var/log/messages", "cros_syslog", false, nil},
		{"/var/log/mount-encrypted.log", "cros_var_log", false, nil},
		{"/var/log/net.log", "cros_net_log", false, nil},
		{"/var/log/secure", "cros_secure_log", false, nil},
		{"/var/log/tlsdate.log", "cros_tlsdate_log", false, nil},
		{"/var/spool", "cros_var_spool", false, nil},
		{"/var/spool/cron-lite", "cros_periodic_scheduler_cache_t", true, nil},
	}

	for _, testArg := range testArgs {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		selinux.CheckContext(s, testArg.path, selinux.S0Object(testArg.context), testArg.recursive, filter)
	}

}
