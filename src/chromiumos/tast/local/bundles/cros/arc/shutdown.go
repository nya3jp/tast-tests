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
		SoftwareDeps: []string{"android", "chrome"},
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

	miniContainerMounts := map[string]struct{}{
		"/opt/google/containers/android/rootfs/root":                        {},
		"/opt/google/containers/arc-obb-mounter/mountpoints/container-root": {},
		"/opt/google/containers/arc-sdcard/mountpoints/container-root":      {},
		"/run/arc/adbd":            {},
		"/run/arc/debugfs/tracing": {},
		"/run/arc/media":           {},
		"/run/arc/obb":             {},
		"/run/arc/oem":             {},
		"/run/arc/sdcard":          {},
		"/run/arc/shared_mounts":   {},
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
