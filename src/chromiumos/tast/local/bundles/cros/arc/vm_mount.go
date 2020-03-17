// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		// TODO: hopefully merge into arc.StartStop once this test becomes stable.
		Func:         VMMount,
		Desc:         "Checks whether ARCVM has healthy mount points after boot",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.VMBooted(),
		Timeout:      4 * time.Minute,
	})
}

func VMMount(ctx context.Context, s *testing.State) {
	ms, err := mountInfoForARCVM(ctx)
	if err != nil {
		s.Fatal("Failed to get mount info for ARCVM: ", err)
	}
	// TODO: add more checks.
	// Check whether these paths are successfully unmounted.
	unmountedPaths := map[string]struct{}{
		"/proc/sys/fs/binfmt_misc": {},
	}
	for _, m := range ms {
		if _, ok := unmountedPaths[m.MountPath]; ok {
			s.Fatalf("Failure: %q is not unmounted", m.MountPath)
		}
	}
}

// mountInfoForARCVM returns a list of mount point info for ARCVM via android-sh.
func mountInfoForARCVM(ctx context.Context) ([]sysutil.MountInfo, error) {
	// Read the mountinfo file through 'android-sh -c cat'.
	cmd := arc.BootstrapCommand(ctx, "/system/bin/cat", "/proc/1/mountinfo")
	mi, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mount info through android-sh")
	}
	var result []sysutil.MountInfo
	for _, line := range strings.Split(string(mi), "\n") {
		if line == "" {
			continue
		}
		info, err := sysutil.ParseMountInfoLine(line)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse mount info")
		}
		result = append(result, info)
	}
	return result, nil
}
