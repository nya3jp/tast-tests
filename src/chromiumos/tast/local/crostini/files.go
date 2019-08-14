// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RemoveContainerFile removes a file from the container's file system. This is
// useful for cleaning up files placed in the container by
// TransferToContainerOrDie().
func RemoveContainerFile(ctx context.Context, cont *vm.Container, containerPath string) error {
	return cont.Command(ctx, "sh", "-c", shutil.EscapeSlice([]string{"sudo", "rm", containerPath})).Run(testexec.DumpLogOnError)
}

// TransferToContainerOrDie copies a file from the host's filesystem to the
// containers.
func TransferToContainerOrDie(ctx context.Context, s *testing.State, cont *vm.Container, hostPath, containerPath string) {
	parentPath := filepath.Dir(containerPath)
	if err := cont.Command(ctx, "mkdir", "-p", parentPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to mkdir %q before pushing: %v", parentPath, err)
	}
	if err := cont.PushFile(ctx, hostPath, containerPath); err != nil {
		s.Fatalf("Failed to push %q to container: %v", hostPath, err)
	}
}

// TransferToContainerAsRootOrDie is similar to TransferToContainerOrDie() but
// works as the root user.
func TransferToContainerAsRootOrDie(ctx context.Context, s *testing.State, cont *vm.Container, hostPath, containerPath string) {
	// TODO(hollingum): modifying PushFile to work as root is very flaky,
	// investigate fixing that so we can avoid this two-phase transfer.
	tempPath := "/tmp/temp_transfer_file"
	TransferToContainerOrDie(ctx, s, cont, hostPath, tempPath)

	if err := cont.Command(ctx, "sh", "-c", shutil.EscapeSlice([]string{"sudo", "mv", tempPath, containerPath})).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to move temporary to %q: %v", containerPath, err)
	}
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
