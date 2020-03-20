// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BinfmtMisc,
		Desc:         "Checks whether binfmt_misc is successfully registered and unmounted in the boot process",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Data:         []string{"hello_world_arm", "hello_world_arm64"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          arc.VMBooted(),
		Timeout:      5 * time.Minute,
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

	// Check whether native bridge is enabled.
	const nativeBridgeProp = "ro.boot.native_bridge"
	nb, err := a.GetProp(ctx, nativeBridgeProp)
	if err != nil {
		s.Fatalf("Failed to getprop %q: %v", nativeBridgeProp, err)
	}
	if nb == "libhoudini.so" || nb == "libndk_translation.so" {
		// Check whether ARM executables can be run.
		if err := pushAndExecute(ctx, a, s.DataPath("hello_world_arm")); err != nil {
			s.Fatal("Failed to run ARM executable: ", err)
		}
		const cpuAbilist64Prop = "ro.product.cpu.abilist64"
		abi, err := a.GetProp(ctx, cpuAbilist64Prop)
		if err != nil {
			s.Fatalf("Failed to getprop %q: %v", cpuAbilist64Prop, err)
		}
		// TODO(youkichihosoi): update the condition once a dedicated property for ARM 64-bit support is implemented.
		if abi == "x86_64,arm64-v8a" {
			if err := pushAndExecute(ctx, a, s.DataPath("hello_world_arm64")); err != nil {
				s.Fatal("Failed to run ARM 64-bit executable: ", err)
			}
		}
	}
}

// pushAndExecute pushes an executable to Android's temporary directory and executes it.
func pushAndExecute(ctx context.Context, a *arc.ARC, execPath string) (retErr error) {
	tmpExecPath, err := a.PushFileToTmpDir(ctx, execPath)
	if err != nil {
		return errors.Wrapf(err, "failed to push %q to tmpdir", execPath)
	}
	defer func() {
		if err := a.Command(ctx, "rm", "-f", tmpExecPath).Run(testexec.DumpLogOnError); err != nil {
			if retErr == nil {
				retErr = errors.Wrapf(err, "failed to remove %q", tmpExecPath)
			} else {
				testing.ContextLogf(ctx, "Failed to remove %q: %v", tmpExecPath, err)
			}
		}
	}()

	if err := a.Command(ctx, "chmod", "0755", tmpExecPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to change the permission of %q", tmpExecPath)
	}
	if err := a.Command(ctx, tmpExecPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to execute %q", tmpExecPath)
	}
	return nil
}

// mountInfoForARCVM returns a list of mount point info for ARCVM.
func mountInfoForARCVM(ctx context.Context, a *arc.ARC) ([]sysutil.MountInfo, error) {
	const mountInfoPath = "/proc/1/mountinfo"
	mi, err := a.ReadFile(ctx, mountInfoPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read mount info file %q", mountInfoPath)
	}
	return sysutil.ParseMountInfo(mi)
}
