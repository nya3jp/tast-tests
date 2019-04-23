// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Shutdown,
		Desc: "Verifies clean shutdown of CrOS Chrome and Android container",
		Contacts: []string{
			"rohitbm@chromium.org", // Original author.
			"arc-eng@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func Shutdown(ctx context.Context, s *testing.State) {
	func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()
	}()

	// Remember the PID of ARC's init, emulate logout, then re-check
	// ARC's init. Note that chrome.Close() above does not log out.
	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to find PID for init: ", err)
	}

	// TODO(rohitbm): identify browser crash using session manager.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// If err != nil, it means ARC is not running, so it's an expected
	// case.
	newPID, err := arc.InitPID()
	if err == nil && newPID == oldPID {
		s.Fatal("ARC was not relaunched. Got PID: ", oldPID)
	}

	// Make sure that ARC related mount points are released, except
	// ones for Mini container.
	ms, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		s.Fatal("Failed to get mount info: ", err)
	}

	isARCMount := func(path string) bool {
		return strings.HasPrefix(path, "/opt/google/containers/android/") ||
			strings.HasPrefix(path, "/opt/google/containers/arc-") ||
			strings.HasPrefix(path, "/run/arc/")
	}

	miniContainerMounts := map[string]int{
		"/opt/google/containers/android/rootfs/root":                        0,
		"/opt/google/containers/arc-obb-mounter/mountpoints/container-root": 0,
		"/opt/google/containers/arc-sdcard/mountpoints/container-root":      0,
		"/run/arc/adbd":            0,
		"/run/arc/debugfs/tracing": 0,
		"/run/arc/media":           0,
		"/run/arc/obb":             0,
		"/run/arc/oem":             0,
		"/run/arc/sdcard":          0,
		"/run/arc/shared_mounts":   0,
	}

	for _, m := range ms {
		if !isARCMount(m.MountPath) {
			continue
		}
		if _, ok := miniContainerMounts[m.MountPath]; !ok {
			s.Error("Mountpoint leaked after logout: ", m.MountPath)
		}
	}
}
