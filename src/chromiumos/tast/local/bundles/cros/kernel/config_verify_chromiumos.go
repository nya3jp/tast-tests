// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"

	"chromiumos/tast/local/bundles/cros/kernel/kernelcommon"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConfigVerifyChromiumOS,
		Desc: "Examines a kernel build CONFIG list to make sure various things are present, missing, built as modules, etc for ChromiumOS",
		Contacts: []string{
			"jeffxu@chromium.org",
			"chromeos-kernel-test@google.com",
			"oka@chromium.org", // Tast port author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chromeos_kernelci"},
	})
}

// ConfigVerifyChromiumOS reads the Linux kernel version and arch to verify validity of
// the information returned depending on version.
func ConfigVerifyChromiumOS(ctx context.Context, s *testing.State) {
	ver, arch, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version and arch: ", err)
	}

	conf, err := kernelcommon.ReadKernelConfig(ctx)
	if err != nil {
		s.Fatal("Failed to read kernel config: ", err)
	}

	kcc := kernelcommon.NewKernelConfigCheck(ver, arch)

	// "ESD_FS" is optional.
	kcc.Optional = append(kcc.Optional, "ESD_FS")

	kcc.Test(conf, s)
}
