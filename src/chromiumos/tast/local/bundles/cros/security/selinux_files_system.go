// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesSystem,
		Desc:         "Checks that SELinux file labels are set correctly for system files",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"chrome", "selinux"},
		Attr:         []string{"group:mainline"},
		Pre:          chrome.LoggedIn(),
	})
}

func SELinuxFilesSystem(ctx context.Context, s *testing.State) {
	gpuDevices, err := selinux.GpuDevices()
	if err != nil {
		// Error instead of Fatal to continue test other testcases .
		// We don't want to "hide" other failures since SELinuxFiles tests are mostly independent test cases.
		s.Error("Failed to enumerate gpu devices: ", err)
	}

	crosEcIioDevices, err := selinux.IIOSensorDevices()
	if err != nil {
		s.Error("Failed to enumerate iio devices: ", err)
	}

	testArgs := []selinux.FileTestCase{
		{Path: "/bin", Context: "cros_coreutils_exec", Recursive: true, Filter: selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{Path: "/bin/bash", Context: "sh_exec"},
		{Path: "/bin/dash", Context: "sh_exec"},
		{Path: "/bin/kmod", Context: "cros_modprobe_exec"},
		{Path: "/bin/sh", Context: "sh_exec"},
		{Path: "/etc", Context: "cros_conf_file", Recursive: true, Filter: selinux.IgnorePaths([]string{
			"/etc/localtime", "/etc/passwd", "/etc/group", "/etc/shadow", "/etc/selinux",
		})},
		{Path: "/etc/group", Context: "cros_passwd_file"},
		{Path: "/etc/localtime", Context: "cros_tz_data_file"},
		{Path: "/etc/passwd", Context: "cros_passwd_file"},
		{Path: "/etc/selinux", Context: "cros_selinux_config_file", Recursive: true},
		{Path: "/etc/shadow", Context: "cros_shadow_file"},
		{Path: "/opt/google/easy_unlock/easy_unlock", Context: "cros_easy_unlock_exec", IgnoreErrors: true, Log: true}, // not found on amd64-generic-full builder. crbug.com/1085813
		{Path: "/run/avahi-daemon", Context: "cros_run_avahi_daemon", Recursive: true, Filter: selinux.IgnorePaths([]string{
			"/run/avahi-daemon/pid", "/run/avahi-daemon/socket",
		})},
		{Path: "/run/avahi-daemon/pid", Context: "cros_avahi_daemon_pid_file", IgnoreErrors: true},
		{Path: "/run/avahi-daemon/socket", Context: "cros_avahi_socket", IgnoreErrors: true},
		{Path: "/run/cras", Context: "cras_socket", Recursive: true},
		{Path: "/run/dbus", Context: "cros_run_dbus"},
		{Path: "/run/dbus.pid", Context: "cros_dbus_daemon_pid_file"},
		{Path: "/run/dbus/system_bus_socket", Context: "cros_system_bus_socket"},
		{Path: "/run/frecon", Context: "cros_run_frecon", Recursive: true},
		{Path: "/run/metrics", Context: "cros_run_metrics"},
		{Path: "/run/metrics/external", Context: "cros_run_metrics_external"},
		{Path: "/run/metrics/external/crash-reporter", Context: "cros_run_metrics_external_crash"},
		{Path: "/run/power_manager", Context: "cros_run_power_manager", Recursive: true},
		{Path: "/run/rsyslogd", Context: "cros_run_rsyslogd"},
		{Path: "/run/udev", Context: "cros_run_udev", Recursive: true, IgnoreErrors: true},
		{Path: "/sbin/chromeos_startup", Context: "chromeos_startup_script_exec"},
		{Path: "/sbin/crash_reporter", Context: "cros_crash_reporter_exec"},
		{Path: "/sbin/crash_sender", Context: "cros_crash_sender_exec"},
		{Path: "/sbin/debugd", Context: "cros_debugd_exec"},
		{Path: "/sbin/dhcpcd", Context: "cros_dhcpcd_exec"},
		{Path: "/sbin/frecon", Context: "frecon_exec"},
		{Path: "/sbin/init", Context: "chromeos_init_exec"},
		{Path: "/sbin/insmod", Context: "cros_modprobe_exec"},
		{Path: "/sbin/minijail0", Context: "cros_minijail_exec"},
		{Path: "/sbin/modprobe", Context: "cros_modprobe_exec"},
		{Path: "/sbin/restorecon", Context: "cros_restorecon_exec"},
		{Path: "/sbin/rmmod", Context: "cros_modprobe_exec"},
		{Path: "/sbin/session_manager", Context: "cros_session_manager_exec"},
		{Path: "/sbin/setfiles", Context: "cros_restorecon_exec"},
		{Path: "/sbin/udevd", Context: "cros_udevd_exec"},
		{Path: "/sbin/upstart-socket-bridge", Context: "upstart_socket_bridge_exec"},
		{Path: "/sys", Context: "sysfs.*", Recursive: true, Filter: selinux.IgnorePathsRegex(append(append([]string{
			"/sys/bus/iio/devices",
			"/sys/class/drm",
			"/sys/devices/system/cpu",
			"/sys/fs/cgroup",
			"/sys/fs/pstore",
			"/sys/fs/selinux",
			"/sys/kernel/config",
			"/sys/kernel/debug",
			// we don't have anything special of conntrack files than others. conntrack slab cache changes when connections established or closes, and may cause flakiness.
			"/sys/kernel/slab/nf_conntrack_.*",
		}, gpuDevices...), crosEcIioDevices...))},
		{Path: "/sys/fs/cgroup", Context: "cgroup", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/cgroup")},
		{Path: "/sys/fs/cgroup", Context: "tmpfs"},
		{Path: "/sys/fs/pstore", Context: "pstorefs"},
		{Path: "/sys/fs/selinux", Context: "selinuxfs", Recursive: true, Filter: selinux.IgnorePathButNotContents("/sys/fs/selinux/null")},
		{Path: "/sys/fs/selinux/null", Context: "null_device"},
		{Path: "/sys/kernel/config", Context: "configfs", IgnoreErrors: true},
		{Path: "/sys/kernel/debug", Context: "debugfs"},
		{Path: "/sys/kernel/debug/debugfs_tracing_on", Context: "debugfs_tracing", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/tracing", Context: "debugfs_tracing"},
		{Path: "/sys/kernel/debug/tracing/trace_marker", Context: "debugfs_trace_marker", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/sync", Context: "debugfs_sync", IgnoreErrors: true},
		{Path: "/sys/kernel/debug/sync/info", Context: "debugfs_sync", IgnoreErrors: true},
		{Path: "/usr/bin", Context: "cros_coreutils_exec", Recursive: true, Filter: selinux.InvertFilterSkipFile(selinux.SkipCoreutilsFile)},
		{Path: "/usr/bin/anomaly_detector", Context: "cros_anomaly_detector_exec"},
		{Path: "/usr/bin/chunneld", Context: "cros_chunneld_exec", IgnoreErrors: true},
		{Path: "/usr/bin/chrt", Context: "cros_chrt_exec"},
		{Path: "/usr/bin/cras", Context: "cros_cras_exec"},
		{Path: "/usr/bin/cups_proxy", Context: "cros_cups_proxy_exec", IgnoreErrors: true},
		{Path: "/usr/bin/dbus-daemon", Context: "cros_dbus_daemon_exec"},
		{Path: "/usr/bin/dbus-uuidgen", Context: "cros_dbus_uuidgen_exec"},
		{Path: "/usr/bin/ionice", Context: "cros_ionice_exec"},
		{Path: "/usr/bin/logger", Context: "cros_logger_exec"},
		{Path: "/usr/bin/memd", Context: "cros_memd_exec"},
		{Path: "/usr/bin/metrics_client", Context: "cros_metrics_client_exec"},
		{Path: "/usr/bin/metrics_daemon", Context: "cros_metrics_daemon_exec"},
		{Path: "/usr/bin/midis", Context: "cros_midis_exec", IgnoreErrors: true},
		{Path: "/usr/bin/mount-passthrough", Context: "cros_mount_passthrough_exec", IgnoreErrors: true},
		{Path: "/usr/bin/mount-passthrough-jailed", Context: "cros_mount_passthrough_jailed_exec", IgnoreErrors: true},
		{Path: "/usr/bin/mount-passthrough-jailed-media", Context: "cros_mount_passthrough_jailed_media_exec", IgnoreErrors: true},
		{Path: "/usr/bin/mount-passthrough-jailed-play", Context: "cros_mount_passthrough_jailed_play_exec", IgnoreErrors: true},
		{Path: "/usr/bin/periodic_scheduler", Context: "cros_periodic_scheduler_exec"},
		{Path: "/usr/bin/powerd", Context: "cros_powerd_exec"},
		{Path: "/usr/bin/resourced", Context: "cros_resourced_exec"},
		{Path: "/usr/bin/seneschal", Context: "cros_seneschal_exec", IgnoreErrors: true},
		{Path: "/usr/bin/shill", Context: "cros_shill_exec"},
		{Path: "/usr/bin/sound_card_init", Context: "cros_sound_card_init_exec", IgnoreErrors: true},
		{Path: "/usr/bin/start_bluetoothd.sh", Context: "cros_init_start_bluetoothd_shell_script"},
		{Path: "/usr/bin/start_bluetoothlog.sh", Context: "cros_init_start_bluetoothlog_shell_script"},
		{Path: "/usr/bin/tlsdated", Context: "cros_tlsdated_exec"},
		{Path: "/usr/bin/tpm2-simulator", Context: "cros_tpm2_simulator_exec", IgnoreErrors: true},
		{Path: "/usr/bin/traced", Context: "cros_traced_exec"},
		{Path: "/usr/bin/traced_probes", Context: "cros_traced_probes_exec"},
		{Path: "/usr/bin/virtual-file-provider", Context: "cros_virtual_file_provider_exec", IgnoreErrors: true},
		{Path: "/usr/bin/virtual-file-provider-jailed", Context: "cros_virtual_file_provider_jailed_exec", IgnoreErrors: true},
		{Path: "/usr/bin/vm_cicerone", Context: "cros_vm_cicerone_exec", IgnoreErrors: true},
		{Path: "/usr/bin/vm_concierge", Context: "cros_vm_concierge_exec", IgnoreErrors: true},
		{Path: "/usr/bin/vmlog_forwarder", Context: "cros_vmlog_forwarder_exec", IgnoreErrors: true},
		{Path: "/usr/libexec/bluetooth/bluetoothd", Context: "cros_bluetoothd_exec"},
		{Path: "/usr/sbin/ModemManager", Context: "cros_modem_manager_exec"},
		{Path: "/usr/sbin/accelerator-logs", Context: "cros_accelerator_logs_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/apk-cache-cleaner-jailed", Context: "cros_apk_cache_cleaner_jailed_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/arc-setup", Context: "cros_arc_setup_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/avahi-daemon", Context: "cros_avahi_daemon_exec"},
		{Path: "/usr/sbin/bootstat", Context: "cros_bootstat_exec"},
		{Path: "/usr/sbin/chapsd", Context: "cros_chapsd_exec"},
		{Path: "/usr/sbin/chromeos-cleanup-logs", Context: "cros_chromeos_cleanup_logs_exec"},
		{Path: "/usr/sbin/chromeos-trim", Context: "cros_chromeos_trim_exec"},
		{Path: "/usr/sbin/conntrackd", Context: "cros_conntrackd_exec"},
		{Path: "/usr/sbin/crosdns", Context: "cros_crosdns_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/cros-machine-id-regen", Context: "cros_machine_id_regen_exec"},
		{Path: "/usr/sbin/cryptohomed", Context: "cros_cryptohomed_exec"},
		{Path: "/usr/sbin/cryptohome-proxy", Context: "cros_cryptohome_proxy_exec"},
		{Path: "/usr/sbin/jetstream-update-stats", Context: "cros_jetstream_update_stats_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/rsyslogd", Context: "cros_rsyslogd_exec"},
		{Path: "/usr/sbin/sshd", Context: "cros_sshd_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/sslh", Context: "cros_sslh_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/sslh-fork", Context: "cros_sslh_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/sslh-select", Context: "cros_sslh_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/tcsd", Context: "cros_tcsd_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/update_engine", Context: "cros_update_engine_exec"},
		{Path: "/usr/sbin/usbguard-daemon", Context: "cros_usbguard_exec", IgnoreErrors: true},
		{Path: "/usr/sbin/wpa_supplicant", Context: "cros_wpa_supplicant_exec"},
		{Path: "/usr/share/cros/init", Context: "cros_init_shell_scripts", Recursive: true, Filter: selinux.IgnorePathsButNotContents([]string{
			"/usr/share/cros/init/activate_date.sh",
			"/usr/share/cros/init/crx-import.sh",
			"/usr/share/cros/init/lockbox-cache.sh",
			"/usr/share/cros/init/powerd-pre-start.sh",
			"/usr/share/cros/init/shill.sh",
			"/usr/share/cros/init/shill-pre-start.sh",
			"/usr/share/cros/init/temp_logger.sh",
			"/usr/share/cros/init/ui-pre-start",
			"/usr/share/cros/init/ui-respawn",
		})},
		{Path: "/usr/share/cros/init/activate_date.sh", Context: "cros_init_activate_date_script", IgnoreErrors: true},
		{Path: "/usr/share/cros/init/crx-import.sh", Context: "cros_init_crx_import_script"},
		{Path: "/usr/share/cros/init/lockbox-cache.sh", Context: "cros_init_lockbox_cache_script"},
		{Path: "/usr/share/cros/init/powerd-pre-start.sh", Context: "cros_init_powerd_pre_start_script"},
		{Path: "/usr/share/cros/init/shill.sh", Context: "cros_init_shill_shell_script"},
		{Path: "/usr/share/cros/init/shill-pre-start.sh", Context: "cros_init_shill_shell_script"},
		{Path: "/usr/share/cros/init/temp_logger.sh", Context: "cros_init_temp_logger_shell_script"},
		{Path: "/usr/share/cros/init/ui-pre-start", Context: "cros_init_ui_pre_start_shell_script"},
		{Path: "/usr/share/cros/init/ui-respawn", Context: "cros_init_ui_respawn_shell_script"},
		{Path: "/usr/share/policy", Context: "cros_seccomp_policy_file", Recursive: true},
		{Path: "/usr/share/userfeedback", Context: "cros_userfeedback_file", Recursive: true},
		{Path: "/var", Context: "cros_var", Log: true},
		{Path: "/var/cache", Context: "cros_var_cache", Log: true},
		{Path: "/var/cache/shill", Context: "cros_var_cache_shill"},
		{Path: "/var/cache/modem-utilities", Context: "cros_var_cache_modem_utilities"},
		{Path: "/var/lib", Context: "cros_var_lib", Log: true},
		{Path: "/var/lib/chaps", Context: "cros_var_lib_chaps", Recursive: true},
		{Path: "/var/lib/crash_reporter", Context: "cros_var_lib_crash_reporter", Recursive: true},
		{Path: "/var/lib/dbus", Context: "cros_var_lib_dbus", Recursive: true},
		{Path: "/var/lib/dhcpcd", Context: "cros_var_lib_shill", Recursive: true},
		{Path: "/var/lib/metrics", Context: "cros_metrics_file", Recursive: true, Filter: selinux.IgnorePathButNotContents("/var/lib/metrics/uma-events")},
		{Path: "/var/lib/metrics/uma-events", Context: "cros_metrics_uma_events_file"},
		{Path: "/var/lib/power_manager", Context: "cros_var_lib_power_manager", Recursive: true},
		{Path: "/var/lib/shill", Context: "cros_var_lib_shill", Recursive: true},
		{Path: "/var/lib/update_engine", Context: "cros_var_lib_update_engine", Recursive: true},
		{Path: "/var/lib/whitelist", Context: "cros_var_lib_whitelist", Recursive: true},
		{Path: "/var/log", Context: "cros_var_log", Log: true},
		{Path: "/var/log/asan", Context: "cros_var_log_asan", Recursive: true, Log: true},
		{Path: "/var/log/authpolicy.log", Context: "cros_authpolicy_log", Log: true},
		{Path: "/var/log/eventlog.txt", Context: "cros_var_log_eventlog", Log: true},
		{Path: "/var/log/mount-encrypted.log", Context: "cros_var_log", IgnoreErrors: true, Log: true},
		{Path: "/var/log/tlsdate.log", Context: "cros_tlsdate_log", Log: true},
		{Path: "/var/spool", Context: "cros_var_spool", Log: true},
		{Path: "/var/spool/crash", Context: "cros_crash_spool", Recursive: true, IgnoreErrors: true, Log: true},
		{Path: "/var/spool/cron-lite", Context: "cros_periodic_scheduler_cache_t", Recursive: true, Log: true},
	}

	selinux.FilesTestInternal(ctx, s, testArgs)
}
