// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
)

// RemoveContainerFile removes a file from the container's file system. This is
// useful for cleaning up files placed in the container by TransferToContainer().
func RemoveContainerFile(ctx context.Context, cont *vm.Container, containerPath string) error {
	return cont.Command(ctx, "sh", "-c", shutil.EscapeSlice([]string{"sudo", "rm", containerPath})).Run(testexec.DumpLogOnError)
}

// TransferToContainer copies a file from the host's filesystem to the containers.
func TransferToContainer(ctx context.Context, cont *vm.Container, hostPath, containerPath string) error {
	parentPath := filepath.Dir(containerPath)
	if err := cont.Command(ctx, "mkdir", "-p", parentPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to mkdir %q before pushing", parentPath)
	}
	return cont.PushFile(ctx, hostPath, containerPath)
}

// TransferToContainerAsRoot is similar to TransferToContainer() but works as the root user.
func TransferToContainerAsRoot(ctx context.Context, cont *vm.Container, hostPath, containerPath string) error {
	parentPath := filepath.Dir(containerPath)
	if err := cont.Command(ctx, "sh", "-c", shutil.EscapeSlice([]string{"sudo", "mkdir", "-p", parentPath})).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to mkdir %q before pushing", parentPath)
	}
	// TODO(hollingum): modifying PushFile to work as root is very flaky,
	// investigate fixing that so we can avoid this two-phase transfer.
	tempPath := "/tmp/temp_transfer_file"
	if err := TransferToContainer(ctx, cont, hostPath, tempPath); err != nil {
		return errors.Wrapf(err, "failed to transfer to a temporary location %q", tempPath)
	}
	return cont.Command(ctx, "sh", "-c", shutil.EscapeSlice([]string{"sudo", "mv", tempPath, containerPath})).Run(testexec.DumpLogOnError)
}

// VerifyFileInContainer checks for file existance in the container, returning
// an error if it fails (or fails to find it).
func VerifyFileInContainer(ctx context.Context, cont *vm.Container, containerPath string) error {
	return cont.Command(ctx, "sh", "-c", "[ -f "+containerPath+" ]").Run()
}

// VerifyFileNotInContainer works much like the above, but for absent files.
func VerifyFileNotInContainer(ctx context.Context, cont *vm.Container, containerPath string) error {
	return cont.Command(ctx, "sh", "-c", "[ ! -f "+containerPath+" ]").Run()
}
