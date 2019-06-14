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

	"chromiumos/tast/errors"
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
	VMPath      string
	DiskPaths   []string
	RWDiskPaths []string
	KernelArgs  []string
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

// WaitForOutput waits until a line matched by re has been written to ch,
// crosvm's stdout is closed, or the deadline is reached. It returns the full
// line that was matched.
func (vm *Crosvm) WaitForOutput(re *regexp.Regexp) (string, error) {

	// Start a goroutine that reads bytes from crosvm and writes them to a channel.
	// We can't do this with lines because then we will miss the initial prompt
	// that comes up that doesn't have a line terminator.
	ch := make(chan byte)
	errch := make(chan error)
	go func() {
		defer close(ch)
		r := bufio.NewReader(vm.stdout)
		for {
			b, err := r.ReadByte()
			if err == io.EOF {
				break
			} else if err != nil {
				errch <- err
			}
			ch <- b
		}
	}()

	var line strings.Builder
	for {
		select {
		case c, more := <-ch:
			if !more {
				return "", errors.New("eof")
			}
			if c == '\n' {
				line.Reset()
			} else {
				line.WriteByte(c)
			}
			if re.MatchString(line.String()) {
				return line.String(), nil
			}
		case err := <-errch:
			return "", err
		}
	}
}
