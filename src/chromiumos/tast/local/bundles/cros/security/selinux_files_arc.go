// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesARC,
		Desc:         "Checks SELinux labels on ARC-specific files on devices that support ARC",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"android", "selinux", "chrome"},
		Pre:          arc.Booted(),
		Attr:         []string{"group:mainline"},
	})
}

type arcFileTestCase struct {
	path          string
	isAndroidPath bool
	context       string
	recursive     bool
	filter        selinux.FileLabelCheckFilter // nil is selinux.CheckAll
}

func SELinuxFilesARC(ctx context.Context, s *testing.State) {
	containerPIDFiles, err := filepath.Glob("/run/containers/android*/container.pid")
	if err != nil {
		s.Fatal("Failed to find container.pid file: ", err)
	}
	if len(containerPIDFiles) != 1 {
		s.Fatal("Expected to find one container.pid file; got ", containerPIDFiles)
	}
	containerPIDFileName := containerPIDFiles[0]

	b, err := ioutil.ReadFile(containerPIDFileName)
	if err != nil {
		s.Fatal("Failed to read container.pid: ", err)
	}
	androidRoot := fmt.Sprintf("/proc/%s/root", strings.TrimSpace(string(b)))

	var testArgs []arcFileTestCase

	gpuDevices, err := selinux.GpuDevices()
	if err != nil {
		// Error instead of Fatal to continue test other testcases .
		// We don't want to "hide" other failures since SELinuxFiles tests are mostly independent test cases.
		s.Error("Failed to enumerate gpu devices: ", err)
	}
	for _, gpuDevice := range gpuDevices {
		testArgs = append(testArgs,
			[]arcFileTestCase{
				{path: filepath.Join(gpuDevice, "config"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: filepath.Join(gpuDevice, "device"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: filepath.Join(gpuDevice, "drm"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: filepath.Join(gpuDevice, "subsystem_device"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: filepath.Join(gpuDevice, "subsystem_vendor"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: filepath.Join(gpuDevice, "uevent"), context: "gpu_device"},
				{path: filepath.Join(gpuDevice, "vendor"), context: "gpu_device", filter: selinux.SkipNotExist},
				{path: gpuDevice, context: "sysfs", recursive: true, filter: selinux.IgnorePaths([]string{
					filepath.Join(gpuDevice, "config"),
					filepath.Join(gpuDevice, "device"),
					filepath.Join(gpuDevice, "drm"),
					filepath.Join(gpuDevice, "subsystem_device"),
					filepath.Join(gpuDevice, "subsystem_vendor"),
					filepath.Join(gpuDevice, "uevent"),
					filepath.Join(gpuDevice, "vendor"),
				})},
			}...,
		)
	}

	iioDevices, err := selinux.IIOSensorDevices()
	if err != nil {
		s.Error("Failed to enumerate iio devices: ", err)
	}
	for _, iioDevice := range iioDevices {
		testArgs = append(
			testArgs,
			[]arcFileTestCase{
				{path: iioDevice, context: "cros_sensor_hal_sysfs", recursive: true, filter: selinux.IIOSensorFilter},
				{path: iioDevice, context: "sysfs", recursive: true, filter: selinux.InvertFilterSkipFile(selinux.IIOSensorFilter)},
			}...)
	}

	testArgs = append(testArgs, []arcFileTestCase{
		{path: "/mnt/stateful_partition/unencrypted/apkcache", context: "apkcache_file"},
		{path: "/mnt/stateful_partition/unencrypted/art-data/dalvik-cache/", context: "dalvikcache_data_file", recursive: true},
		{path: "/opt/google/chrome/chrome", context: "chrome_browser_exec"},
		{path: "/run/arc/adbd", context: "device"},
		{path: "/run/arc/bugreport", context: "debug_bugreport"},
		{path: "/run/arc/bugreport/pipe", context: "debug_bugreport"},
		{path: "/run/arc/cmdline.android", context: "(proc_cmdline|proc)"}, // N or below is proc
		{path: "/run/arc/debugfs", context: "(debugfs|tmpfs)"},
		{path: "/run/arc/fake_kptr_restrict", context: "proc_security"},
		{path: "/run/arc/fake_mmap_rnd_bits", context: "proc_security"},
		{path: "/run/arc/fake_mmap_rnd_compat_bits", context: "proc_security"},
		{path: "/run/arc/media", context: "tmpfs"},
		{path: "/run/arc/obb", context: "tmpfs"},
		{path: "/run/arc/oem/etc", context: "oemfs", recursive: true},
		{path: "/run/arc/properties/build.prop", context: "system_file"},
		{path: "/run/arc/properties/default.prop", context: "rootfs"},
		{path: "/run/arc/sdcard", context: "storage_file"},
		{path: "/run/arc/shared_mounts", context: "tmpfs"},
		{path: "/run/camera", context: "(camera_dir|camera_socket)"}, // N or below is camera_socket
		{path: "/run/camera/camera.sock", context: "camera_socket", filter: selinux.SkipNotExist},
		{path: "/run/camera/camera3.sock", context: "camera_socket", filter: selinux.SkipNotExist},
		{path: "/run/chrome/arc_bridge.sock", context: "arc_bridge_socket"},
		{path: "/run/chrome/wayland-0", context: "wayland_socket"},
		{path: "/run/cras", context: "cras_socket", recursive: true},
		{path: "/run/session_manager", context: "cros_run_session_manager", recursive: true},
		{path: "/sys/kernel/debug/sync/sw_sync", context: "debugfs_sw_sync", filter: selinux.SkipNotExist},
		{path: "/usr/sbin/arc-setup", context: "cros_arc_setup_exec"},
		{path: "/var/log/chrome", context: "cros_var_log_chrome", recursive: true},
		{path: "dev/ptmx", isAndroidPath: true, context: "ptmx_device"},
		{path: "dev/random", isAndroidPath: true, context: "random_device"},
		{path: "dev/urandom", isAndroidPath: true, context: "u?random_device"},
		{path: "oem", isAndroidPath: true, context: "oemfs"},
		{path: "sys/kernel/debug/sync", isAndroidPath: true, context: "tmpfs|debugfs_sync"}, // pre-3.18 doesn't have debugfs/sync, thus ARC container has a tmpfs fake.
	}...)

	for _, testArg := range testArgs {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		path := testArg.path
		if testArg.isAndroidPath {
			path = filepath.Join(androidRoot, path)
		}
		expected, err := selinux.FileContextRegexp(testArg.context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testArg.context, err)
			continue
		}
		selinux.CheckContext(ctx, s, &selinux.CheckContextReq{
			Path:      path,
			Expected:  expected,
			Recursive: testArg.recursive,
			Filter:    filter,
			Log:       false,
		})
	}
	selinux.CheckHomeDirectory(ctx, s)
}
