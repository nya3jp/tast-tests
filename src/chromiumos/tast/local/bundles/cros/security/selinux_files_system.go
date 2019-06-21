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
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"informational"},
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

	// Files to be tested.
	// Files should have been labeled by platform2/sepolicy/file_contexts/ or
	// platform2/sepolicy/policy/*/genfs_contexts with a few exceptions.
	// Exceptions include:
	//  - type_transition rule to default assign a label for files created
	// under some condition.
	//  - mv/cp files without preserving original labels but inheriting
	// labels from new parent directory (e.g. /var/log/mount-encrypted.log)
	testArgs := []struct {
		path      string // absolute file path
		context   string // expected SELinux file context
		recursive bool
		filter    selinux.FileLabelCheckFilter
		log       bool
	}{
		{"/bin", "cros_coreutils_exec", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile), false},
		{"/bin/bash", "sh_exec", false, nil, false},
		{"/bin/dash", "sh_exec", false, nil, false},
		{"/bin/kmod", "cros_modprobe_exec", false, nil, false},
		{"/bin/sh", "sh_exec", false, nil, false},
		{"/etc", "cros_conf_file", true, selinux.IgnorePaths([]string{
			"/etc/localtime", "/etc/passwd", "/etc/group", "/etc/shadow", "/etc/selinux",
		}), false},
		{"/etc/group", "cros_passwd_file", false, nil, false},
		{"/etc/localtime", "cros_tz_data_file", false, nil, false},
		{"/etc/passwd", "cros_passwd_file", false, nil, false},
		{"/etc/selinux", "cros_selinux_config_file", true, nil, false},
		{"/etc/shadow", "cros_shadow_file", false, nil, false},
		{"/run/avahi-daemon", "cros_run_avahi_daemon", true, selinux.IgnorePaths([]string{
			"/run/avahi-daemon/pid", "/run/avahi-daemon/socket",
		}), false},
		{"/run/avahi-daemon/pid", "cros_avahi_daemon_pid_file", false, selinux.SkipNotExist, false},
		{"/run/avahi-daemon/socket", "cros_avahi_socket", false, selinux.SkipNotExist, false},
		{"/run/cras", "cras_socket", true, nil, false},
		{"/run/dbus", "cros_run_dbus", false, nil, false},
		{"/run/dbus.pid", "cros_dbus_daemon_pid_file", false, nil, false},
		{"/run/dbus/system_bus_socket", "cros_system_bus_socket", false, nil, false},
		{"/run/frecon", "cros_run_frecon", true, nil, false},
		{"/run/power_manager", "cros_run_power_manager", true, nil, false},
		{"/run/udev", "cros_run_udev", true, selinux.SkipNotExist, false},
		{"/sbin/chromeos_startup", "chromeos_startup_script_exec", false, nil, false},
		{"/sbin/crash_reporter", "cros_crash_reporter_exec", false, nil, false},
		{"/sbin/crash_sender", "cros_crash_sender_exec", false, nil, false},
		{"/sbin/debugd", "cros_debugd_exec", false, nil, false},
		{"/sbin/dhcpcd", "cros_dhcpcd_exec", false, nil, false},
		{"/sbin/frecon", "frecon_exec", false, nil, false},
		{"/sbin/init", "chromeos_init_exec", false, nil, false},
		{"/sbin/insmod", "cros_modprobe_exec", false, nil, false},
		{"/sbin/minijail0", "cros_minijail_exec", false, nil, false},
		{"/sbin/modprobe", "cros_modprobe_exec", false, nil, false},
		{"/sbin/restorecon", "cros_restorecon_exec", false, nil, false},
		{"/sbin/rmmod", "cros_modprobe_exec", false, nil, false},
		{"/sbin/session_manager", "cros_session_manager_exec", false, nil, false},
		{"/sbin/setfiles", "cros_restorecon_exec", false, nil, false},
		{"/sbin/udevd", "cros_udevd_exec", false, nil, false},
		{"/sbin/upstart-socket-bridge", "upstart_socket_bridge_exec", false, nil, false},
		{"/sys/devices/system/cpu", "sysfs", true, systemCPUFilter(writable), false},
		{"/sys/devices/system/cpu", "sysfs_devices_system_cpu", true, systemCPUFilter(readonly), false},
		{"/sys/fs/cgroup", "cgroup", true, selinux.IgnorePathButNotContents("/sys/fs/cgroup"), false},
		{"/sys/fs/cgroup", "tmpfs", false, nil, false},
		{"/sys/fs/pstore", "pstorefs", false, nil, false},
		{"/sys/fs/selinux", "selinuxfs", true, selinux.IgnorePathButNotContents("/sys/fs/selinux/null"), false},
		{"/sys/fs/selinux/null", "null_device", false, nil, false},
		{"/sys/kernel/config", "configfs", false, selinux.SkipNotExist, false},
		{"/sys/kernel/debug", "debugfs", false, nil, false},
		{"/sys/kernel/debug/debugfs_tracing_on", "debugfs_tracing", false, selinux.SkipNotExist, false},
		{"/sys/kernel/debug/tracing", "debugfs_tracing", false, nil, false},
		{"/sys/kernel/debug/tracing/trace_marker", "debugfs_trace_marker", false, selinux.SkipNotExist, false},
		{"/usr/bin", "cros_coreutils_exec", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile), false},
		{"/usr/bin/anomaly_detector", "cros_anomaly_detector_exec", false, nil, false},
		{"/usr/bin/chrt", "cros_chrt_exec", false, nil, false},
		{"/usr/bin/cras", "cros_cras_exec", false, nil, false},
		{"/usr/bin/dbus-daemon", "cros_dbus_daemon_exec", false, nil, false},
		{"/usr/bin/dbus-uuidgen", "cros_dbus_uuidgen_exec", false, nil, false},
		{"/usr/bin/ionice", "cros_ionice_exec", false, nil, false},
		{"/usr/bin/logger", "cros_logger_exec", false, nil, false},
		{"/usr/bin/memd", "cros_memd_exec", false, nil, false},
		{"/usr/bin/metrics_client", "cros_metrics_client_exec", false, nil, false},
		{"/usr/bin/metrics_daemon", "cros_metrics_daemon_exec", false, nil, false},
		{"/usr/bin/midis", "cros_midis_exec", false, selinux.SkipNotExist, false},
		{"/usr/bin/periodic_scheduler", "cros_periodic_scheduler_exec", false, nil, false},
		{"/usr/bin/powerd", "cros_powerd_exec", false, nil, false},
		{"/usr/bin/shill", "cros_shill_exec", false, nil, false},
		{"/usr/bin/start_bluetoothd.sh", "cros_init_start_bluetoothd_shell_script", false, nil, false},
		{"/usr/bin/start_bluetoothlog.sh", "cros_init_start_bluetoothlog_shell_script", false, nil, false},
		{"/usr/bin/tlsdated", "cros_tlsdated_exec", false, nil, false},
		{"/usr/libexec/bluetooth/bluetoothd", "cros_bluetoothd_exec", false, nil, false},
		{"/usr/sbin/ModemManager", "cros_modem_manager_exec", false, nil, false},
		{"/usr/sbin/accelerator-logs", "cros_accelerator_logs_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/apk-cache-cleaner-jailed", "cros_apk_cache_cleaner_jailed_exec", false, nil, false},
		{"/usr/sbin/arc-setup", "cros_arc_setup_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/avahi-daemon", "cros_avahi_daemon_exec", false, nil, false},
		{"/usr/sbin/bootstat", "cros_bootstat_exec", false, nil, false},
		{"/usr/sbin/chapsd", "cros_chapsd_exec", false, nil, false},
		{"/usr/sbin/chromeos-cleanup-logs", "cros_chromeos_cleanup_logs_exec", false, nil, false},
		{"/usr/sbin/chromeos-trim", "cros_chromeos_trim_exec", false, nil, false},
		{"/usr/sbin/conntrackd", "cros_conntrackd_exec", false, nil, false},
		{"/usr/sbin/cros-machine-id-regen", "cros_machine_id_regen_exec", false, nil, false},
		{"/usr/sbin/cryptohomed", "cros_cryptohomed_exec", false, nil, false},
		{"/usr/sbin/jetstream-update-stats", "cros_jetstream_update_stats_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/rsyslogd", "cros_rsyslogd_exec", false, nil, false},
		{"/usr/sbin/sshd", "cros_sshd_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/sslh", "cros_sslh_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/sslh-fork", "cros_sslh_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/sslh-select", "cros_sslh_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/tcsd", "cros_tcsd_exec", false, selinux.SkipNotExist, false},
		{"/usr/sbin/update_engine", "cros_update_engine_exec", false, nil, false},
		{"/usr/sbin/wpa_supplicant", "cros_wpa_supplicant_exec", false, nil, false},
		{"/usr/share/cros/init", "cros_init_shell_scripts", true, selinux.IgnorePathsButNotContents([]string{
			"/usr/share/cros/init/activate_date.sh",
			"/usr/share/cros/init/crx-import.sh",
			"/usr/share/cros/init/lockbox-cache.sh",
			"/usr/share/cros/init/powerd-pre-start.sh",
			"/usr/share/cros/init/shill.sh",
			"/usr/share/cros/init/shill-pre-start.sh",
			"/usr/share/cros/init/ui-pre-start",
			"/usr/share/cros/init/ui-respawn",
		}), false},
		{"/usr/share/cros/init/activate_date.sh", "cros_init_activate_date_script", false, selinux.SkipNotExist, false},
		{"/usr/share/cros/init/crx-import.sh", "cros_init_crx_import_script", false, nil, false},
		{"/usr/share/cros/init/lockbox-cache.sh", "cros_init_lockbox_cache_script", false, nil, false},
		{"/usr/share/cros/init/powerd-pre-start.sh", "cros_init_powerd_pre_start_script", false, nil, false},
		{"/usr/share/cros/init/shill.sh", "cros_init_shill_shell_script", false, nil, false},
		{"/usr/share/cros/init/shill-pre-start.sh", "cros_init_shill_shell_script", false, nil, false},
		{"/usr/share/cros/init/ui-pre-start", "cros_init_ui_pre_start_shell_script", false, nil, false},
		{"/usr/share/cros/init/ui-respawn", "cros_init_ui_respawn_shell_script", false, nil, false},
		{"/usr/share/policy", "cros_seccomp_policy_file", true, nil, false},
		{"/usr/share/userfeedback", "cros_userfeedback_file", true, nil, false},
		{"/var", "cros_var", false, nil, true},
		{"/var/cache", "cros_var_cache", false, nil, true},
		{"/var/cache/shill", "cros_var_cache_shill", false, nil, false},
		{"/var/empty", "cros_var_empty", false, nil, false},
		{"/var/lib", "cros_var_lib", false, nil, true},
		{"/var/lib/chaps", "cros_var_lib_chaps", true, nil, false},
		{"/var/lib/crash_reporter", "cros_var_lib_crash_reporter", true, nil, false},
		{"/var/lib/dbus", "cros_var_lib_dbus", true, nil, false},
		{"/var/lib/dhcpcd", "cros_var_lib_shill", true, nil, false},
		{"/var/lib/metrics", "cros_metrics_file", true, selinux.IgnorePathButNotContents("/var/lib/metrics/uma-events"), false},
		{"/var/lib/metrics/uma-events", "cros_metrics_uma_events_file", false, nil, false},
		{"/var/lib/power_manager", "cros_var_lib_power_manager", true, nil, false},
		{"/var/lib/shill", "cros_var_lib_shill", true, nil, false},
		{"/var/lib/update_engine", "cros_var_lib_update_engine", true, nil, false},
		{"/var/lib/whitelist", "cros_var_lib_whitelist", true, nil, false},
		{"/var/log", "cros_var_log", false, nil, true},
		{"/var/log/arc.log", "cros_arc_log", false, nil, true},
		{"/var/log/authpolicy.log", "cros_authpolicy_log", false, nil, true},
		{"/var/log/boot.log", "cros_boot_log", false, nil, true},
		{"/var/log/eventlog.txt", "cros_var_log_eventlog", false, nil, true},
		{"/var/log/messages", "cros_syslog", false, nil, true},
		{"/var/log/mount-encrypted.log", "cros_var_log", false, nil, true},
		{"/var/log/net.log", "cros_net_log", false, nil, true},
		{"/var/log/secure", "cros_secure_log", false, nil, true},
		{"/var/log/tlsdate.log", "cros_tlsdate_log", false, nil, true},
		{"/var/log/asan", "cros_var_log_asan", true, nil, true},
		{"/var/spool", "cros_var_spool", false, nil, true},
		{"/var/spool/crash", "cros_crash_spool", true, selinux.SkipNotExist, true},
		{"/var/spool/cron-lite", "cros_periodic_scheduler_cache_t", true, nil, true},
	}

	for _, testArg := range testArgs {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		expected, err := selinux.FileContextRegexp(testArg.context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testArg.context, err)
			continue
		}
		selinux.CheckContext(ctx, s, testArg.path, expected, testArg.recursive, filter, testArg.log)
	}

}
