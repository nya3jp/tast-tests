// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FileSystemPermissions,
		Desc: "Verifies filesystem permissions for ARC container",
		Contacts: []string{
			"yusukes@chromium.org",
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func FileSystemPermissions(ctx context.Context, s *testing.State) {
	// Android UID/GID inside of the container.
	const (
		aidRoot    = "0"
		aidSystem  = "1000"
		aidCache   = "2001"
		aidUnknown = "65534"
	)

	a := s.PreValue().(arc.PreData).ARC
	arcPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get ARC init PID: ", err)
	}
	mounts, err := sysutil.MountInfoForPID(int(arcPID))
	if err != nil {
		s.Fatal("Failed to get mount info for ARC: ", err)
	}

	expectMounts := map[string]sysutil.MountOpt{
		"/":         sysutil.MntReadonly | sysutil.MntNodev,
		"/data":     sysutil.MntNosuid | sysutil.MntNodev,
		"/dev/ptmx": sysutil.MntNoexec | sysutil.MntNosuid,
		"/dev":      sysutil.MntNosuid,
		"/proc":     sysutil.MntNosuid | sysutil.MntNodev | sysutil.MntNoexec,
	}
	expectPerms := []struct {
		path string
		uid  string
		gid  string
		perm string
	}{
		{"/", aidRoot, aidRoot, "755"},
		{"/data", aidSystem, aidSystem, "771"},
		{"/dev/pts/ptmx", aidRoot, aidRoot, "666"},
		{"/dev/ptmx", aidRoot, aidRoot, "666"},
		{"/dev", aidRoot, aidRoot, "755"},
		{"/proc", aidUnknown, aidUnknown, "555"},
		{"/sys/kernel/debug", aidRoot, aidRoot, "755"},
		{"/sys/kernel/debug/tracing", aidUnknown, aidUnknown, "755"},
	}

	// TODO(arc-storage): Add tests once /data/cache is correctly mounted.
	if ver, err := arc.SDKVersion(); err != nil {
		s.Fatal("Failed to get ARC SDK version: ", err)
	} else if ver == arc.SDKN {
		expectMounts["/cache"] = sysutil.MntNoexec | sysutil.MntNosuid | sysutil.MntNodev
		expectPerms = append(expectPerms, struct {
			path string
			uid  string
			gid  string
			perm string
		}{"/cache", aidSystem, aidCache, "770"})
	}

	for _, m := range mounts {
		// When boot-continue is in use, the container's mountinfo has
		// two entries for /cache. One is for the read-only /cache
		// directory for mini container, and the other is for the real
		// writable /cache.
		// The former is hidden by the latter, and is not accessible
		// but the proc file still shows both. We should ignore the
		// former so that the auto test can check the visible and
		// writable one. Do the same for /data. Note that /dev/root is
		// the read-only Chrome OS root fs.
		if m.MountSource == "/dev/root" && (m.MountPath == "/cache" || m.MountPath == "/data") {
			continue
		}

		e, ok := expectMounts[m.MountPath]
		if !ok {
			continue
		}
		opts := m.MountOpts &^ (sysutil.MntNoatime | sysutil.MntRelatime)
		if opts != e {
			s.Errorf("Unexpected mount opt at %s: got %d; want %d", m.MountPath, opts, e)
		}
	}

	for _, e := range expectPerms {
		if out, err := a.Command(ctx, "stat", "-c", "%a", e.path).Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to get permission at %s: %v", e.path, err)
		} else if val := strings.TrimSpace(string(out)); val != e.perm {
			s.Errorf("Unexpected perm at %s: got %q; want %q", e.path, val, e.perm)
		}

		if out, err := a.Command(ctx, "stat", "-c", "%u", e.path).Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to get UID at %s: %v", e.path, err)
		} else if val := strings.TrimSpace(string(out)); val != e.uid {
			s.Errorf("Unexpected uid at %s: got %q; want %q", e.path, val, e.uid)
		}

		if out, err := a.Command(ctx, "stat", "-c", "%g", e.path).Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to get GID at %s: %v", e.path, err)
		} else if val := strings.TrimSpace(string(out)); val != e.gid {
			s.Errorf("Unexpected gid at %s: got %q; want %q", e.path, val, e.gid)
		}
	}
}
