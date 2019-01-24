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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesARC,
		Desc:         "Checks SELinux labels on ARC-specific files on devices that support ARC",
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"android", "selinux", "chrome_login"},
	})
}

func SELinuxFilesARC(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

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

	for _, testArg := range []struct {
		path          string
		isAndroidPath bool
		context       string
		recursive     bool
		filter        selinux.FileLabelCheckFilter // nil is selinux.CheckAll
	}{
		// TODO(fqj): Missing file tests from cheets_SELinux*.py are:
		// _check_drm_render_sys_devices_labels
		// _check_iio_sys_devices_labels
		// _check_misc_sys_labels
		// _check_sys_kernel_debug_labels (debugfs/sync missing)
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
		{"/usr/sbin/arc-setup", false, "cros_arc_setup_exec", false, nil},
		{"/var/log/chrome", false, "cros_var_log_chrome", true, nil},
		{"dev/ptmx", true, "ptmx_device", false, nil},
		{"dev/random", true, "random_device", false, nil},
		{"dev/urandom", true, "u?random_device", false, nil},
		{"oem", true, "oemfs", false, nil},
	} {
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
		selinux.CheckContext(s, path, expected, testArg.recursive, filter)
	}
	selinux.CheckHomeDirectory(s)
}
