// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"chromiumos/tast/testing"
)

// CrosVM holds the info about a running instace of the crosvm command.
type CrosVM struct {
	crosVM     *exec.Cmd
	socketPath string
	stdin      io.WriteCloser
	stdout     io.ReadCloser
}

// StartVM starts a crosvm instance with the optional disk path as an additional disk.
func StartVM(ctx context.Context, diskPath string, kernelArgs []string) (*CrosVM, error) {
	componentPath, err := LoadTerminaComponent(ctx)
	if err != nil {
		return nil, err
	}

	sp, err := genSocketPath()
	if err != nil {
		return nil, err
	}
	args := []string{"run", "--socket", sp, "--root", filepath.Join(componentPath, "vm_rootfs.img")}
	if diskPath != "" {
		args = append(args, "--rwdisk", diskPath)
	}
	args = append(args, kernelArgs...)
	args = append(args, filepath.Join(componentPath, "vm_kernel"))

	c := exec.Command("crosvm", args...)
	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err = c.Start(); err != nil {
		return nil, err
	}

	return &CrosVM {
		crosVM:     c,
		socketPath: sp,
		stdin:      stdin,
		stdout:     stdout,
	}, nil
}

// Close stops a VM that was returned from StartVM.
func (r *CrosVM) Close(ctx context.Context) error {
	err := exec.Command("crosvm", "stop", r.socketPath).Run()
	if err != nil {
		testing.ContextLogf(ctx, "failed to exec stop")
		return err
	}
	return r.crosVM.Wait()
}

// ExecCommand starts a command in the VM by sending it to stdin.
func (r *CrosVM) ExecCommand(c string) error {
	if _, err := r.stdin.Write([]byte(c + "\n")); err != nil {
		return fmt.Errorf("failed to write command: %v", err)
	}
	return nil
}

// WaitPrompt waits for the command prompt to be returned by the VM. If a
// command prompt is found in the output, it returns true, otherwise false. In
// addition all output leading up to the command prompt is returned.
func (r *CrosVM) WaitPrompt(ctx context.Context) (bool, *bytes.Buffer, error) {
	var output bytes.Buffer
	tee := io.TeeReader(r.stdout, &output)
	matched, err := regexp.MatchReader("localhost.+#", bufio.NewReader(io.LimitReader(tee, 16384)))
	if err != nil {
		return false, nil, err
	}
	return matched, &output, nil
}

// RunCommand runs the given command and waits for the shell prompt to be
// returned. The output from the command is returned.
func (r *CrosVM) RunCommand(ctx context.Context, cmd string) (*bytes.Buffer, error) {
	err := r.ExecCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("Run command failed:", cmd)
	}

	gotPrompt, cmdOutput, err := r.WaitPrompt(ctx)
	if err != nil {
		return nil, fmt.Errorf("Run command completion failed:", cmd)
	}
	if !gotPrompt {
		return nil, fmt.Errorf("Failed to find command prompt after: ", cmd)
	}
	return cmdOutput, nil
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
