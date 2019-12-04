// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package startstop

import (
	"context"
	"strings"

	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// TestMount runs inside arc.StartStop.
type TestMount struct{}

// PreStart implements Fixture.PreStart().
func (*TestMount) PreStart(ctx context.Context, s *testing.State) {
	// Do nothing.
}

// PostStart implements Fixture.PostStart().
func (*TestMount) PostStart(ctx context.Context, s *testing.State) {
	// Do nothing.
}

// PostStop implements Fixture.PostStop(). It makes sure that ARC related
// mount points are released, except ones for Mini container.
func (*TestMount) PostStop(ctx context.Context, s *testing.State) {
	ms, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		s.Error("Failed to get mount info: ", err)
		return
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
