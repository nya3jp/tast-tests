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
		Func:         BinfmtMisc,
		Desc:         "Checks whether binfmt_misc is successfully registered and unmounted after ARCVM boot",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm_r"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          arc.VMBooted(),
		Timeout:      5 * time.Minute,
	})
}

func BinfmtMisc(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	ms, err := mountInfoForARCVM(ctx, a)
	if err != nil {
		s.Fatal("Failed to get mount info for ARCVM: ", err)
	}
	const binfmtMiscPath = "/proc/sys/fs/binfmt_misc"
	for _, m := range ms {
		if m.MountPath == binfmtMiscPath {
			s.Fatalf("Failure: %q is not unmounted", binfmtMiscPath)
		}
	}
	const nativeBridgeProp = "ro.boot.native_bridge"
	nb, err := a.GetProp(ctx, nativeBridgeProp)
	if err != nil {
		s.Fatalf("Failed to getprop %q: %v", nativeBridgeProp, err)
	}
	if nb == "libhoudini.so" || nb == "libndk_translation.so" {
		const armExe = "/system/bin/arm/linker"
		if err := a.Command(ctx, armExe).Run(); err != nil {
			s.Fatalf("Failed to execute %q: %v", armExe, err)
		}
		// TODO(youkichihosoi): update once a dedicated property for ARM 64-bit support is implemented.
		const cpuAbilist64Prop = "ro.product.cpu.abilist64"
		abi, err := a.GetProp(ctx, cpuAbilist64Prop)
		if err != nil {
			s.Fatalf("Failed to getprop %q: %v", cpuAbilist64Prop, err)
		}
		if abi == "x86_64,arm64-v8a" {
			const arm64Exe = "/system/bin/arm64/linker64"
			if err := a.Command(ctx, arm64Exe).Run(); err != nil {
				s.Fatalf("Failed to execute %q: %v", arm64Exe, err)
			}
		}
	}
}

// mountInfoForARCVM returns a list of mount point info for ARCVM via ADB.
func mountInfoForARCVM(ctx context.Context, a *arc.ARC) ([]sysutil.MountInfo, error) {
	cmd := a.Command(ctx, "/system/bin/cat", "/proc/1/mountinfo")
	mi, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get mount info via ADB")
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
