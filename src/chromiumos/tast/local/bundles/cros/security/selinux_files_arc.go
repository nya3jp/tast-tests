// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesArc,
		Desc:         "Checks that ARC++ related file SELinux contexts.",
		SoftwareDeps: []string{"arc", "selinux"},
	})
}

func SELinuxFilesArc(ctx context.Context, s *testing.State) {
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

	procs, err := selinux.GetProcesses()
	if err != nil {
		s.Fatal("Failed to list procs: ", procs)
	}
	androidInitProcs := selinux.FindProcessesBySEContext(procs, "u:r:init:s0")
	if len(androidInitProcs) != 1 {
		s.Fatal("Failed to locate android init process")
	}
	androidInitProc := androidInitProcs[0]
	androidRoot := fmt.Sprintf("/proc/%d/root", androidInitProc.PID)

	for _, testArg := range []struct {
		path          string
		isAndroidPath bool
		context       string
		recursive     bool
		filter        selinux.FileLabelCheckFilter
	}{
		// TODO(fqj): Missing file tests from cheets_SELinux*.py are:
		// _check_drm_render_sys_devices_labels
		// _check_iio_sys_devices_labels
		// _check_misc_sys_labels
		// _check_sys_kernel_debug_labels (debugfs/sync missing)
		{"dev/random", true, "random_device", false, nil},
		{"dev/urandom", true, "urandom_device", false, nil},
		{"dev/ptmx", true, "ptmx_device", false, nil},
		{"oem", true, "oemfs", false, nil},
		{"/mnt/stateful_partition/unencrypted/apkcache", false, "apkcache_file", false, nil},
		{"/run/chrome/arc_bridge.sock", false, "arc_bridge_socket", false, nil},
		{"/run/arc/adbd", false, "device", false, nil},
		{"/run/arc/bugreport", false, "debug_bugreport", false, nil},
		{"/run/arc/bugreport/pipe", false, "debug_bugreport", false, nil},
		{"/run/arc/debugfs", false, "(debugfs|tmpfs)", false, nil},
		{"/run/arc/media", false, "tmpfs", false, nil},
		{"/run/arc/obb", false, "tmpfs", false, nil},
		{"/run/arc/sdcard", false, "storage_file", false, nil},
		{"/run/arc/shared_mounts", false, "tmpfs", false, nil},
		{"/run/camera", false, "(camera_dir|camera_socket)", false, nil}, // N or below is camera_socket
		{"/run/camera/camera.sock", false, "camera_socket", false, selinux.SkipNotExist},
		{"/run/camera/camera3.sock", false, "camera_socket", false, selinux.SkipNotExist},
		{"/run/arc/cmdline.android", false, "(proc_cmdline|proc)", false, nil}, // N or below is proc
		{"/run/cras", false, "cras_socket", true, nil},
		{"/mnt/stateful_partition/unencrypted/art-data/dalvik-cache/", false, "dalvikcache_data_file", true, nil},
		{"/run/arc/fake_kptr_restrict", false, "proc_security", false, nil},
		{"/run/arc/fake_mmap_rnd_bits", false, "proc_security", false, nil},
		{"/run/arc/fake_mmap_rnd_compat_bits", false, "proc_security", false, nil},
		{"/run/arc/oem/etc", false, "oemfs", true, nil},
		{"/run/arc/properties/default.prop", false, "rootfs", false, nil},
		{"/run/arc/properties/build.prop", false, "system_file", false, nil},
	} {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		path := testArg.path
		if testArg.isAndroidPath {
			path = fmt.Sprintf("%s/%s", androidRoot, path)
		}
		expected, err := selinux.FileContextRegexp(testArg.context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testArg.context, err)
			continue
		}
		selinux.CheckContext(s, path, expected, testArg.recursive, filter)
	}
}
