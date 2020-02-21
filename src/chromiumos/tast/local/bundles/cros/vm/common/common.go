// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"os"
	"runtime"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// VirtiofsKernel returns the name of the data file that should be used as the guest kernel
// for tests that use virtio-fs.
func VirtiofsKernel() string {
	if runtime.GOARCH == "amd64" {
		return "virtiofs_kernel_x86_64.xz"
	}

	return "virtiofs_kernel_aarch64.xz"
}

// UnpackKernel unpacks an xz compressed kernel image.
func UnpackKernel(ctx context.Context, src, dst string) error {
	testing.ContextLog(ctx, "Unpacking kernel")

	s, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "failed to open kernel source file")
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "failed to create kernel destination file")
	}
	defer d.Close()

	xz := testexec.CommandContext(ctx, "xz", "-d", "-c")
	xz.Stdin = s
	xz.Stdout = d

	if err := xz.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to decompress kernel")
	}

	return nil
}
