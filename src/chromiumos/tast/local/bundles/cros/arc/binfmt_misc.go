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
		Desc:         "Checks whether binfmt_misc is successfully registered and unmounted in the boot process",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Data:         []string{"hello_world_arm", "hello_world_arm64"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Timeout: 5 * time.Minute,
	})
}

func BinfmtMisc(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	// Check whether binfmt_misc is unmounted.
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

	// Check whether ARM executables can be run when native bridge is enabled.
	const nativeBridgeProp = "ro.boot.native_bridge"
	nb, err := a.GetProp(ctx, nativeBridgeProp)
	if err != nil {
		s.Fatalf("Failed to getprop %q: %v", nativeBridgeProp, err)
	}
	if nb == "libhoudini.so" || nb == "libndk_translation.so" {
		if err := pushAndRun(ctx, a, s.DataPath("hello_world_arm")); err != nil {
			s.Fatal("Failed to run ARM executable: ", err)
		}
		const cpuAbilist64Prop = "ro.product.cpu.abilist64"
		abi, err := a.GetProp(ctx, cpuAbilist64Prop)
		if err != nil {
			s.Fatalf("Failed to getprop %q: %v", cpuAbilist64Prop, err)
		}
		// TODO(youkichihosoi): update the condition once a dedicated property for ARM 64-bit support is implemented.
		if abi == "x86_64,arm64-v8a" {
			if err := pushAndRun(ctx, a, s.DataPath("hello_world_arm64")); err != nil {
				s.Fatal("Failed to run ARM 64-bit executable: ", err)
			}
		}
	}
}

// pushAndRun pushes an executable to Android's temporary directory and runs it.
func pushAndRun(ctx context.Context, a *arc.ARC, execPath string) error {
	tmpExecPath, err := a.PushFileToTmpDir(ctx, execPath)
	if err != nil {
		errors.Wrapf(err, "failed to push %q to tmpdir", execPath)
	}
	if err := a.Command(ctx, tmpExecPath).Run(); err != nil {
		errors.Wrapf(err, "failed to execute %q", tmpExecPath)
	}
	return nil
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
