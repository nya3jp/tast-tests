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
		Func:         SELinuxFilesSystem,
		Desc:         "Checks that SELinux file labels are set correctly for system files",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFilesSystem(ctx context.Context, s *testing.State) {
	type rwFilter int
	const (
		readonly rwFilter = iota
		writable
	)
	systemCPUFilter := func(writableFilter rwFilter) selinux.FileLabelCheckFilter {
		return func(p string, fi os.FileInfo) (skipFile, skipSubdir selinux.FilterResult) {
			mode := fi.Mode()
			// Domain has search to both sysfs and sysfs_devices_system_cpu.
			if mode.IsDir() {
				return selinux.Skip, selinux.Check
			}

			isWritable := mode.IsRegular() && ((mode.Perm() & (syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)) > 0)
			// Writable files
			if isWritable != (writableFilter == writable) {
				return selinux.Skip, selinux.Check
			}

			return selinux.Check, selinux.Check
		}
	}

	testArgs := []selinux.FileTestCase{
		{Path: "/bin", Context: "cros_coreutils_exec", Recursive: true, Filter: selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile), Log: false},
		{Path: "/bin/bash", Context: "sh_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/bin/dash", Context: "sh_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/bin/kmod", Context: "cros_modprobe_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/bin/sh", Context: "sh_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/etc", Context: "cros_conf_file", Recursive: true, Filter: selinux.IgnorePaths([]string{
			"/etc/localtime", "/etc/passwd", "/etc/group", "/etc/shadow", "/etc/selinux",
		}), Log: false},
		{Path: "/etc/group", Context: "cros_passwd_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/etc/localtime", Context: "cros_tz_data_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/etc/passwd", Context: "cros_passwd_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/etc/selinux", Context: "cros_selinux_config_file", Recursive: true, Filter: nil, Log: false},
		{Path: "/etc/shadow", Context: "cros_shadow_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/avahi-daemon", Context: "cros_run_avahi_daemon", Recursive: true, Filter: selinux.IgnorePaths([]string{
			"/run/avahi-daemon/pid", "/run/avahi-daemon/socket",
		}), Log: false},
		{Path: "/run/avahi-daemon/pid", Context: "cros_avahi_daemon_pid_file", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/run/avahi-daemon/socket", Context: "cros_avahi_socket", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/run/cras", Context: "cras_socket", Recursive: true, Filter: nil, Log: false},
		{Path: "/run/dbus", Context: "cros_run_dbus", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/dbus.pid", Context: "cros_dbus_daemon_pid_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/dbus/system_bus_socket", Context: "cros_system_bus_socket", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/frecon", Context: "cros_run_frecon", Recursive: true, Filter: nil, Log: false},
		{Path: "/run/metrics", Context: "cros_run_metrics", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/metrics/external", Context: "cros_run_metrics_external", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/metrics/external/crash-reporter", Context: "cros_run_metrics_external_crash", Recursive: false, Filter: nil, Log: false},
		{Path: "/run/power_manager", Context: "cros_run_power_manager", Recursive: true, Filter: nil, Log: false},
		{Path: "/run/udev", Context: "cros_run_udev", Recursive: true, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/sbin/chromeos_startup", Context: "chromeos_startup_script_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/crash_reporter", Context: "cros_crash_reporter_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/crash_sender", Context: "cros_crash_sender_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/debugd", Context: "cros_debugd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/dhcpcd", Context: "cros_dhcpcd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/frecon", Context: "frecon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/init", Context: "chromeos_init_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/insmod", Context: "cros_modprobe_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/minijail0", Context: "cros_minijail_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/modprobe", Context: "cros_modprobe_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/restorecon", Context: "cros_restorecon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/rmmod", Context: "cros_modprobe_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/session_manager", Context: "cros_session_manager_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/setfiles", Context: "cros_restorecon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/udevd", Context: "cros_udevd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sbin/upstart-socket-bridge", Context: "upstart_socket_bridge_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/devices/system/cpu", Context: "sysfs", Recursive: true, Filter: systemCPUFilter(writable), Log: false},
		{Path: "/sys/devices/system/cpu", Context: "sysfs_devices_system_cpu", Recursive: true, Filter: systemCPUFilter(readonly), Log: false},
		{Path: "/sys/fs/cgroup", Context: "cgroup", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/cgroup"), Log: false},
		{Path: "/sys/fs/cgroup", Context: "tmpfs", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/fs/pstore", Context: "pstorefs", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/fs/selinux", Context: "selinuxfs", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/selinux/null"), Log: false},
		{Path: "/sys/fs/selinux/null", Context: "null_device", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/kernel/config", Context: "configfs", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/sys/kernel/debug", Context: "debugfs", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/kernel/debug/debugfs_tracing_on", Context: "debugfs_tracing", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/sys/kernel/debug/tracing", Context: "debugfs_tracing", Recursive: false, Filter: nil, Log: false},
		{Path: "/sys/kernel/debug/tracing/trace_marker", Context: "debugfs_trace_marker", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/bin", Context: "cros_coreutils_exec", Recursive: true, Filter: selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile), Log: false},
		{Path: "/usr/bin/anomaly_detector", Context: "cros_anomaly_detector_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/chrt", Context: "cros_chrt_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/cras", Context: "cros_cras_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/dbus-daemon", Context: "cros_dbus_daemon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/dbus-uuidgen", Context: "cros_dbus_uuidgen_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/ionice", Context: "cros_ionice_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/logger", Context: "cros_logger_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/memd", Context: "cros_memd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/metrics_client", Context: "cros_metrics_client_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/metrics_daemon", Context: "cros_metrics_daemon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/midis", Context: "cros_midis_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/bin/periodic_scheduler", Context: "cros_periodic_scheduler_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/powerd", Context: "cros_powerd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/shill", Context: "cros_shill_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/start_bluetoothd.sh", Context: "cros_init_start_bluetoothd_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/start_bluetoothlog.sh", Context: "cros_init_start_bluetoothlog_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/bin/tlsdated", Context: "cros_tlsdated_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/libexec/bluetooth/bluetoothd", Context: "cros_bluetoothd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/ModemManager", Context: "cros_modem_manager_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/accelerator-logs", Context: "cros_accelerator_logs_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/apk-cache-cleaner-jailed", Context: "cros_apk_cache_cleaner_jailed_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/arc-setup", Context: "cros_arc_setup_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/avahi-daemon", Context: "cros_avahi_daemon_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/bootstat", Context: "cros_bootstat_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/chapsd", Context: "cros_chapsd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/chromeos-cleanup-logs", Context: "cros_chromeos_cleanup_logs_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/chromeos-trim", Context: "cros_chromeos_trim_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/conntrackd", Context: "cros_conntrackd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/cros-machine-id-regen", Context: "cros_machine_id_regen_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/cryptohomed", Context: "cros_cryptohomed_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/jetstream-update-stats", Context: "cros_jetstream_update_stats_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/rsyslogd", Context: "cros_rsyslogd_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/sshd", Context: "cros_sshd_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/sslh", Context: "cros_sslh_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/sslh-fork", Context: "cros_sslh_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/sslh-select", Context: "cros_sslh_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/tcsd", Context: "cros_tcsd_exec", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/sbin/update_engine", Context: "cros_update_engine_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/sbin/wpa_supplicant", Context: "cros_wpa_supplicant_exec", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init", Context: "cros_init_shell_scripts", Recursive: true, Filter: selinux.IgnorePathsButNotContents([]string{
			"/usr/share/cros/init/activate_date.sh",
			"/usr/share/cros/init/crx-import.sh",
			"/usr/share/cros/init/lockbox-cache.sh",
			"/usr/share/cros/init/powerd-pre-start.sh",
			"/usr/share/cros/init/shill.sh",
			"/usr/share/cros/init/shill-pre-start.sh",
			"/usr/share/cros/init/ui-pre-start",
			"/usr/share/cros/init/ui-respawn",
		}), Log: false},
		{Path: "/usr/share/cros/init/activate_date.sh", Context: "cros_init_activate_date_script", Recursive: false, Filter: selinux.SkipNotExist, Log: false},
		{Path: "/usr/share/cros/init/crx-import.sh", Context: "cros_init_crx_import_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/lockbox-cache.sh", Context: "cros_init_lockbox_cache_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/powerd-pre-start.sh", Context: "cros_init_powerd_pre_start_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/shill.sh", Context: "cros_init_shill_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/shill-pre-start.sh", Context: "cros_init_shill_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/ui-pre-start", Context: "cros_init_ui_pre_start_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/cros/init/ui-respawn", Context: "cros_init_ui_respawn_shell_script", Recursive: false, Filter: nil, Log: false},
		{Path: "/usr/share/policy", Context: "cros_seccomp_policy_file", Recursive: true, Filter: nil, Log: false},
		{Path: "/usr/share/userfeedback", Context: "cros_userfeedback_file", Recursive: true, Filter: nil, Log: false},
		{Path: "/var", Context: "cros_var", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/cache", Context: "cros_var_cache", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/cache/shill", Context: "cros_var_cache_shill", Recursive: false, Filter: nil, Log: false},
		{Path: "/var/empty", Context: "cros_var_empty", Recursive: false, Filter: nil, Log: false},
		{Path: "/var/lib", Context: "cros_var_lib", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/lib/chaps", Context: "cros_var_lib_chaps", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/crash_reporter", Context: "cros_var_lib_crash_reporter", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/dbus", Context: "cros_var_lib_dbus", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/dhcpcd", Context: "cros_var_lib_shill", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/metrics", Context: "cros_metrics_file", Recursive: true, Filter: selinux.IgnorePathButNotContents("/var/lib/metrics/uma-events"), Log: false},
		{Path: "/var/lib/metrics/uma-events", Context: "cros_metrics_uma_events_file", Recursive: false, Filter: nil, Log: false},
		{Path: "/var/lib/power_manager", Context: "cros_var_lib_power_manager", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/shill", Context: "cros_var_lib_shill", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/update_engine", Context: "cros_var_lib_update_engine", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/lib/whitelist", Context: "cros_var_lib_whitelist", Recursive: true, Filter: nil, Log: false},
		{Path: "/var/log", Context: "cros_var_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/asan", Context: "cros_var_log_asan", Recursive: true, Filter: nil, Log: true},
		{Path: "/var/log/authpolicy.log", Context: "cros_authpolicy_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/eventlog.txt", Context: "cros_var_log_eventlog", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/mount-encrypted.log", Context: "cros_var_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/log/tlsdate.log", Context: "cros_tlsdate_log", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/spool", Context: "cros_var_spool", Recursive: false, Filter: nil, Log: true},
		{Path: "/var/spool/crash", Context: "cros_crash_spool", Recursive: true, Filter: selinux.SkipNotExist, Log: true},
		{Path: "/var/spool/cron-lite", Context: "cros_periodic_scheduler_cache_t", Recursive: true, Filter: nil, Log: true},
	}

	selinux.FilesTestInternal(ctx, s, testArgs)
}
