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
				{filepath.Join(gpuDevice, "config"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "device"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "drm"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "subsystem_device"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "subsystem_vendor"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "uevent"), false, "gpu_device", false, nil},
				{filepath.Join(gpuDevice, "vendor"), false, "gpu_device", false, nil},
				{gpuDevice, false, "sysfs", true, selinux.IgnorePaths([]string{
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
				{iioDevice, false, "cros_sensor_hal_sysfs", true, selinux.IIOSensorFilter},
				{iioDevice, false, "sysfs", true, selinux.InvertFilterSkipFile(selinux.IIOSensorFilter)},
			}...)
	}

	testArgs = append(testArgs, []arcFileTestCase{
		{"/mnt/stateful_partition/unencrypted/apkcache", false, "apkcache_file", false, nil},
		{"/mnt/stateful_partition/unencrypted/art-data/dalvik-cache/", false, "dalvikcache_data_file", true, nil},
		{"/opt/google/chrome/chrome", false, "chrome_browser_exec", false, nil},
		{"/run/arc/adbd", false, "device", false, nil},
		{"/run/arc/bugreport", false, "debug_bugreport", false, nil},
		{"/run/arc/bugreport/pipe", false, "debug_bugreport", false, nil},
		{"/run/arc/cmdline.android", false, "(proc_cmdline|proc)", false, nil}, // N or below is proc
		{"/run/arc/debugfs", false, "(debugfs|tmpfs)", false, nil},
		{"/run/arc/fake_kptr_restrict", false, "proc_security", false, nil},
		{"/run/arc/fake_mmap_rnd_bits", false, "proc_security", false, nil},
		{"/run/arc/fake_mmap_rnd_compat_bits", false, "proc_security", false, nil},
		{"/run/arc/media", false, "tmpfs", false, nil},
		{"/run/arc/obb", false, "tmpfs", false, nil},
		{"/run/arc/oem/etc", false, "oemfs", true, nil},
		{"/run/arc/properties/build.prop", false, "system_file", false, nil},
		{"/run/arc/properties/default.prop", false, "rootfs", false, nil},
		{"/run/arc/sdcard", false, "storage_file", false, nil},
		{"/run/arc/shared_mounts", false, "tmpfs", false, nil},
		{"/run/camera", false, "(camera_dir|camera_socket)", false, nil}, // N or below is camera_socket
		{"/run/camera/camera.sock", false, "camera_socket", false, selinux.SkipNotExist},
		{"/run/camera/camera3.sock", false, "camera_socket", false, selinux.SkipNotExist},
		{"/run/chrome/arc_bridge.sock", false, "arc_bridge_socket", false, nil},
		{"/run/chrome/wayland-0", false, "wayland_socket", false, nil},
		{"/run/cras", false, "cras_socket", true, nil},
		{"/run/session_manager", false, "cros_run_session_manager", true, nil},
		{"/sys/kernel/debug/sync/sw_sync", false, "debugfs_sw_sync", false, selinux.SkipNotExist},
		{"/usr/sbin/arc-setup", false, "cros_arc_setup_exec", false, nil},
		{"/var/log/chrome", false, "cros_var_log_chrome", true, nil},
		{"dev/ptmx", true, "ptmx_device", false, nil},
		{"dev/random", true, "random_device", false, nil},
		{"dev/urandom", true, "u?random_device", false, nil},
		{"oem", true, "oemfs", false, nil},
		{"sys/kernel/debug/sync", true, "tmpfs|debugfs_sync", false, nil}, // pre-3.18 doesn't have debugfs/sync, thus ARC container has a tmpfs fake.
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
		selinux.CheckContext(ctx, s, path, expected, testArg.recursive, filter, false)
	}
	selinux.CheckHomeDirectory(ctx, s)
}
