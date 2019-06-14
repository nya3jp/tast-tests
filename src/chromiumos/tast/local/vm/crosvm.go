// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// CrosvmParams - Parameters for starting a crosvm instance.
type CrosvmParams struct {
	VMPath      string   // file path where VM rootfs and kernel are stored
	DiskPaths   []string // paths that will be mounted read only
	RWDiskPaths []string // paths that will be mounted read/write
	KernelArgs  []string // string arguments to be passed to the VM kernel
}

// NewCrosvm starts a crosvm instance with the optional disk path as an additional disk.
func NewCrosvm(ctx context.Context, params *CrosvmParams) (*Crosvm, error) {
	var err error
	if _, err = os.Stat(params.VMPath); os.IsNotExist(err) {
		return nil, err
	}

	vm := &Crosvm{}

	if vm.socketPath, err = genSocketPath(); err != nil {
		return nil, err
	}
	args := []string{"run", "--socket", vm.socketPath, "--root",
		filepath.Join(params.VMPath, "vm_rootfs.img")}

	for _, path := range params.RWDiskPaths {
		args = append(args, "--rwdisk", path)
	}

	for _, path := range params.DiskPaths {
		args = append(args, "-d", path)
	}

	for _, arg := range params.KernelArgs {
		args = append(args, "-p", arg)
	}

	args = append(args, filepath.Join(params.VMPath, "vm_kernel"))

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
	defer os.Remove(file.Name())
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

// WaitForOutput waits until a line matched by re has been written to stdout,
// crosvm's stdout is closed, or the deadline is reached. It returns the full
// line that was matched.
func (vm *Crosvm) WaitForOutput(ctx context.Context, re *regexp.Regexp) (string, error) {
	// Start a goroutine that reads bytes from crosvm and buffers them in a
	// string builder. We can't do this with lines because then we will miss the
	// initial prompt that comes up that doesn't have a line terminator. If a
	// matching line is found, send it through the channel.
	ch := make(chan string)
	errch := make(chan error)
	go func() {
		defer close(ch)
		var line strings.Builder
		r := bufio.NewReader(vm.stdout)
		for {
			b, err := r.ReadByte()
			if err != nil {
				errch <- err
				break
			} else if b == '\n' {
				line.Reset()
			} else {
				line.WriteByte(b)
			}
			if re.MatchString(line.String()) {
				ch <- line.String()
				break
			}
		}
	}()

	// Wait for the matched line, an error, or the context deadline.
	for {
		select {
		case s := <-ch:
			return s, nil
		case err := <-errch:
			return "", err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
