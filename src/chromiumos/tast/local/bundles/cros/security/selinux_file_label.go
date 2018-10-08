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
		{"/bin", "u:object_r:cros_coreutils_exec:s0", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{"/bin/bash", "u:object_r:sh_exec:s0", false, nil},
		{"/bin/dash", "u:object_r:sh_exec:s0", false, nil},
		{"/bin/kmod", "u:object_r:cros_modprobe_exec:s0", false, nil},
		{"/bin/sh", "u:object_r:sh_exec:s0", false, nil},
		{"/etc/group", "u:object_r:cros_passwd_file:s0", false, nil},
		{"/etc/hosts", "u:object_r:cros_network_conf_file:s0", false, selinux.SkipNotExist},
		{"/etc/hosts.d", "u:object_r:cros_network_conf_file:s0", true, nil},
		{"/etc/init", "u:object_r:cros_init_conf_file:s0", true, nil},
		{"/etc/ld.so.cache", "u:object_r:cros_ld_conf_cache:s0", false, nil},
		{"/etc/ld.so.conf", "u:object_r:cros_ld_conf_cache:s0", false, nil},
		{"/etc/nsswitch.conf", "u:object_r:cros_network_conf_file:s0", false, nil},
		{"/etc/passwd", "u:object_r:cros_passwd_file:s0", false, nil},
		{"/etc/resolv.conf", "u:object_r:cros_network_conf_file:s0", false, nil},
		{"/etc/rsyslog.chromeos", "u:object_r:cros_rsyslog_conf_file:s0", false, nil},
		{"/etc/rsyslog.conf", "u:object_r:cros_rsyslog_conf_file:s0", false, nil},
		{"/etc/rsyslog.d", "u:object_r:cros_rsyslog_conf_file:s0", true, nil},
		{"/run/cras", "u:object_r:cras_socket:s0", true, nil},
		{"/sbin/crash_reporter", "u:object_r:cros_crash_reporter_exec:s0", false, nil},
		{"/sbin/crash_sender", "u:object_r:cros_crash_sender_exec:s0", false, nil},
		{"/sbin/debugd", "u:object_r:cros_debugd_exec:s0", false, nil},
		{"/sbin/frecon", "u:object_r:frecon_exec:s0", false, nil},
		{"/sbin/init", "u:object_r:chromeos_init_exec:s0", false, nil},
		{"/sbin/insmod", "u:object_r:cros_modprobe_exec:s0", false, nil},
		{"/sbin/minijail0", "u:object_r:cros_minijail_exec:s0", false, nil},
		{"/sbin/modprobe", "u:object_r:cros_modprobe_exec:s0", false, nil},
		{"/sbin/rmmod", "u:object_r:cros_modprobe_exec:s0", false, nil},
		{"/sbin/session_manager", "u:object_r:cros_session_manager_exec:s0", false, nil},
		{"/sbin/udevd", "u:object_r:cros_udevd_exec:s0", false, nil},
		{"/sbin/upstart-socket-bridge", "u:object_r:upstart_socket_bridge_exec:s0", false, nil},
		{"/sys/devices/system/cpu", "u:object_r:sysfs:s0", true, selinux.InvertFilterSkipFile(systemCPUFilter)},
		{"/sys/devices/system/cpu", "u:object_r:sysfs_devices_system_cpu:s0", true, systemCPUFilter},
		{"/sys/fs/cgroup", "u:object_r:cgroup:s0", true, selinux.IgnorePathButNotContents("/sys/fs/cgroup")},
		{"/sys/fs/cgroup", "u:object_r:tmpfs:s0", false, nil},
		{"/sys/fs/pstore", "u:object_r:pstorefs:s0", false, nil},
		{"/sys/fs/selinux", "u:object_r:selinuxfs:s0", true, selinux.IgnorePathButNotContents("/sys/fs/selinux/null")},
		{"/sys/fs/selinux/null", "u:object_r:null_device:s0", false, nil},
		{"/sys/kernel/config", "u:object_r:configfs:s0", false, selinux.SkipNotExist},
		{"/sys/kernel/debug", "u:object_r:debugfs:s0", false, nil},
		{"/sys/kernel/debug/debugfs_tracing_on", "u:object_r:debugfs_tracing:s0", false, selinux.SkipNotExist},
		{"/sys/kernel/debug/tracing", "u:object_r:debugfs_tracing:s0", false, nil},
		{"/sys/kernel/debug/tracing/trace_marker", "u:object_r:debugfs_trace_marker:s0", false, selinux.SkipNotExist},
		{"/usr/bin", "u:object_r:cros_coreutils_exec:s0", true, selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{"/usr/bin/anomaly_collector", "u:object_r:cros_anomaly_collector_exec:s0", false, nil},
		{"/usr/bin/chrt", "u:object_r:cros_chrt_exec:s0", false, nil},
		{"/usr/bin/cras", "u:object_r:cros_cras_exec:s0", false, nil},
		{"/usr/bin/dbus-daemon", "u:object_r:cros_dbus_daemon_exec:s0", false, nil},
		{"/usr/bin/ionice", "u:object_r:cros_ionice_exec:s0", false, nil},
		{"/usr/bin/logger", "u:object_r:cros_logger_exec:s0", false, nil},
		{"/usr/bin/memd", "u:object_r:cros_memd_exec:s0", false, nil},
		{"/usr/bin/metrics_daemon", "u:object_r:cros_metrics_daemon_exec:s0", false, nil},
		{"/usr/bin/midis", "u:object_r:cros_midis_exec:s0", false, selinux.SkipNotExist},
		{"/usr/bin/periodic_scheduler", "u:object_r:cros_periodic_scheduler_exec:s0", false, nil},
		{"/usr/bin/powerd", "u:object_r:cros_powerd_exec:s0", false, nil},
		{"/usr/bin/shill", "u:object_r:cros_shill_exec:s0", false, nil},
		{"/usr/bin/start_bluetoothd.sh", "u:object_r:cros_init_start_bluetoothd_shell_script:s0", false, nil},
		{"/usr/bin/tlsdated", "u:object_r:cros_tlsdated_exec:s0", false, nil},
		{"/usr/sbin/ModemManager", "u:object_r:cros_modem_manager_exec:s0", false, nil},
		{"/usr/sbin/avahi-daemon", "u:object_r:cros_avahi_daemon_exec:s0", false, nil},
		{"/usr/sbin/chapsd", "u:object_r:cros_chapsd_exec:s0", false, nil},
		{"/usr/sbin/chromeos-cleanup-logs", "u:object_r:cros_chromeos_cleanup_logs_exec:s0", false, nil},
		{"/usr/sbin/chromeos-trim", "u:object_r:cros_chromeos_trim_exec:s0", false, nil},
		{"/usr/sbin/conntrackd", "u:object_r:cros_conntrackd_exec:s0", false, nil},
		{"/usr/sbin/cros-machine-id-regen", "u:object_r:cros_machine_id_regen_exec:s0", false, nil},
		{"/usr/sbin/cryptohomed", "u:object_r:cros_cryptohomed_exec:s0", false, nil},
		{"/usr/sbin/rsyslogd", "u:object_r:cros_rsyslogd_exec:s0", false, nil},
		{"/usr/sbin/sslh", "u:object_r:cros_sslh_exec:s0", false, selinux.SkipNotExist},
		{"/usr/sbin/sslh-fork", "u:object_r:cros_sslh_exec:s0", false, selinux.SkipNotExist},
		{"/usr/sbin/sslh-select", "u:object_r:cros_sslh_exec:s0", false, selinux.SkipNotExist},
		{"/usr/sbin/update_engine", "u:object_r:cros_update_engine_exec:s0", false, nil},
		{"/usr/sbin/wpa_supplicant", "u:object_r:cros_wpa_supplicant_exec:s0", false, nil},
		{"/usr/share/cros/init", "u:object_r:cros_init_shell_scripts:s0", true, selinux.IgnorePathsButNotContents([]string{"/usr/share/cros/init/ui-pre-start", "/usr/share/cros/init/ui-respawn"})},
		{"/usr/share/cros/init/ui-pre-start", "u:object_r:cros_init_ui_pre_start_shell_script:s0", false, nil},
		{"/usr/share/cros/init/ui-respawn", "u:object_r:cros_init_ui_respawn_shell_script:s0", false, nil},
		{"/var", "u:object_r:cros_var:s0", false, nil},
		{"/var/empty", "u:object_r:cros_var_empty:s0", false, nil},
		{"/var/lib", "u:object_r:cros_var_lib:s0", false, nil},
		{"/var/lib/metrics", "u:object_r:cros_metrics_file:s0", true, selinux.IgnorePathButNotContents("/var/lib/metrics/uma-events")},
		{"/var/lib/metrics/uma-events", "u:object_r:cros_metrics_uma_events_file:s0", false, nil},
		{"/var/log", "u:object_r:cros_var_log:s0", false, nil},
		{"/var/log/arc.log", "u:object_r:cros_arc_log:s0", false, nil},
		{"/var/log/authpolicy.log", "u:object_r:cros_authpolicy_log:s0", false, nil},
		{"/var/log/boot.log", "u:object_r:cros_boot_log:s0", false, nil},
		{"/var/log/messages", "u:object_r:cros_syslog:s0", false, nil},
		{"/var/log/mount-encrypted.log", "u:object_r:cros_var_log:s0", false, nil},
		{"/var/log/net.log", "u:object_r:cros_net_log:s0", false, nil},
		{"/var/log/secure", "u:object_r:cros_secure_log:s0", false, nil},
		{"/var/log/tlsdate.log", "u:object_r:cros_tlsdate_log:s0", false, nil},
		{"/var/spool", "u:object_r:cros_var_spool:s0", false, nil},
		{"/var/spool/cron-lite", "u:object_r:cros_periodic_scheduler_cache_t:s0", true, nil},
	}

	for _, testArg := range testArgs {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		selinux.CheckContext(s, testArg.path, testArg.context, testArg.recursive, filter)
	}

}
