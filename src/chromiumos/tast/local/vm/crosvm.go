// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Crosvm holds info about a running instance of the crosvm command.
type Crosvm struct {
	cmd        *testexec.Cmd // crosvm process
	socketPath string        // crosvm control socket
	stdin      io.Writer     // stdin for cmd
	stdout     *os.File      // stdout for cmd; uses os.File to set a read dealine
}

// CrosvmParams - Parameters for starting a crosvm instance.
type CrosvmParams struct {
	VMKernel    string   // path to the VM kernel image
	RootfsPath  string   // optional path to the VM rootfs
	DiskPaths   []string // paths that will be mounted read only
	RWDiskPaths []string // paths that will be mounted read/write
	KernelArgs  []string // string arguments to be passed to the VM kernel
}

// NewCrosvm starts a crosvm instance with the optional disk path as an additional disk.
func NewCrosvm(ctx context.Context, params *CrosvmParams) (*Crosvm, error) {
	if _, err := os.Stat(params.VMKernel); err != nil {
		return nil, errors.Wrap(err, "failed to find VM kernel")
	}

	vm := &Crosvm{}

	var err error
	if vm.socketPath, err = genSocketPath(); err != nil {
		return nil, err
	}
	args := []string{"run", "--socket", vm.socketPath}

	if params.RootfsPath != "" {
		args = append(args, "--root", params.RootfsPath)
	}

	for _, path := range params.RWDiskPaths {
		args = append(args, "--rwdisk", path)
	}

	for _, path := range params.DiskPaths {
		args = append(args, "-d", path)
	}

	for _, arg := range params.KernelArgs {
		args = append(args, "-p", arg)
	}

	args = append(args, params.VMKernel)

	vm.cmd = testexec.CommandContext(ctx, "crosvm", args...)

	if vm.stdin, err = vm.cmd.StdinPipe(); err != nil {
		return nil, err
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	vm.cmd.Stdout = pw
	vm.stdout = pr

	defer pw.Close()

	if err = vm.cmd.Start(); err != nil {
		pr.Close()
		return nil, err
	}
	return vm, nil
}

// Close stops the crosvm process (and underlying VM) started by NewCrosvm.
func (vm *Crosvm) Close(ctx context.Context) error {
	defer vm.stdout.Close()
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
// line that was matched. This function will consume output from stdout until it
// returns.
func (vm *Crosvm) WaitForOutput(ctx context.Context, re *regexp.Regexp) (string, error) {
	// Start a goroutine that reads bytes from crosvm and buffers them in a
	// string builder. We can't do this with lines because then we will miss the
	// initial prompt that comes up that doesn't have a line terminator. If a
	// matching line is found, send it through the channel.
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	// Allow the blocking read call to stop when the deadline has been exceeded.
	// Defer removing the deadline until this function has exited.
	deadline, ok := ctx.Deadline()
	// If no deadline is set, default to no timeout.
	if !ok {
		deadline = time.Time{}
	}
	vm.stdout.SetReadDeadline(deadline)
	defer vm.stdout.SetReadDeadline(time.Time{})
	go func() {
		defer close(ch)
		var line strings.Builder
		var b [1]byte
		for {
			_, err := vm.stdout.Read(b[:])
			if err != nil {
				ch <- result{"", err}
				return
			}
			if b[0] == '\n' {
				line.Reset()
				continue
			}
			line.WriteByte(b[0])
			if re.MatchString(line.String()) {
				ch <- result{line.String(), nil}
				return
			}
		}
	}()

	select {
	case r := <-ch:
		if os.IsTimeout(r.err) {
			// If the read times out, this means the deadline has passed
			select {
			case <-ctx.Done():
				return "", errors.Wrap(ctx.Err(), "timeout out waiting for output")
			}
		}
		return r.line, r.err
	}
}
