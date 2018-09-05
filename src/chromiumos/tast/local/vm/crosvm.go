// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Crosvm holds info about a running instance of the crosvm command.
type Crosvm struct {
	cmd        *testexec.Cmd // crosvm process
	socketPath string        // crosvm control socket
	stdin      io.Writer     // stdin for cmd
	stdout     io.Reader     // stdout for cmd
}

// NewCrosvm starts a crosvm instance with the optional disk path as an additional disk.
func NewCrosvm(ctx context.Context, diskPath string, kernelArgs []string) (*Crosvm, error) {
	componentPath, err := LoadTerminaComponent(ctx)
	if err != nil {
		return nil, err
	}

	vm := &Crosvm{}

	if vm.socketPath, err = genSocketPath(); err != nil {
		return nil, err
	}
	args := []string{"run", "--socket", vm.socketPath, "--root",
		filepath.Join(componentPath, "vm_rootfs.img")}
	if diskPath != "" {
		args = append(args, "--rwdisk", diskPath)
	}
	args = append(args, kernelArgs...)
	args = append(args, filepath.Join(componentPath, "vm_kernel"))

	vm.cmd = testexec.CommandContext(ctx, "crosvm", args...)

	if vm.stdin, err = vm.cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if vm.stdout, err = vm.cmd.StdoutPipe(); err != nil {
		return nil, err
	}
	if err = vm.cmd.Start(); err != nil {
		return nil, err
	}
	return vm, nil
}

// Close stops the crosvm process (and underlying VM) started by NewCrosvm.
func (vm *Crosvm) Close(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "crosvm", "stop", vm.socketPath)
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to exec stop: ", err)
		cmd.DumpLog(ctx)
		return err
	}
	if err := vm.cmd.Wait(); err != nil {
		testing.ContextLog(ctx, "Failed waiting for crosvm to exit: ", err)
		vm.cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// Stdin is attached to the crosvm process's stdin. It can be used to run commands.
func (vm *Crosvm) Stdin() io.Writer {
	return vm.stdin
}

// Stdout is attached to the crosvm process's stdout. It receives all console output.
func (vm *Crosvm) Stdout() io.Reader {
	return vm.stdout
}

// genSocketPath returns a path suitable to use as a temporary crosvm control socket path.
func genSocketPath() (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "crosvm_socket")
	if err != nil {
		return "", err
	}
	name := file.Name()
	os.Remove(name)
	return name, nil
}
