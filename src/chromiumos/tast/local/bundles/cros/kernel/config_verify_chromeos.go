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
		Func: ConfigVerifyChromeOS,
		Desc: "Examines a kernel build CONFIG list to make sure various things are present, missing, built as modules, etc for ChromeOS",
		Contacts: []string{
			"jeffxu@chromium.org",
			"chromeos-kernel-test@google.com",
			"oka@chromium.org", // Tast port author
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// ConfigVerifyChromeOS reads the Linux kernel version and arch to verify validity of
// the information returned depending on version.
func ConfigVerifyChromeOS(ctx context.Context, s *testing.State) {
	ver, arch, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version and arch: ", err)
	}

	conf, err := kernelcommon.ReadKernelConfig(ctx)
	if err != nil {
		s.Fatal("Failed to read kernel config: ", err)
	}

	kcc := kernelcommon.NewKernelConfigCheck(ver, arch)

	// Security; make sure the ChromeOS LSM is in use.
	kcc.Builtin = append(kcc.Builtin, "SECURITY_CHROMIUMOS")

	// NaCl; allow mprotect+PROT_EXEC on noexec mapped files.
	kcc.Value["MMAP_NOEXEC_TAINT"] = "0"

	if ver.IsOrLater(3, 18) {
		kcc.Builtin = append(kcc.Builtin, "ESD_FS")
	}

	if ver.IsOrLater(4, 4) {
		// Security; prevent overflows that can be checked at compile-time.
		kcc.Builtin = append(kcc.Builtin, "FORTIFY_SOURCE")
	}

	kcc.Test(conf, s)
}
