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
	"regexp"

	"chromiumos/tast/testing"
)

// CrosVM holds the info about a running instace of the crosvm command.
type RunningVM struct {
	ctx        context.Context
	crosVM     *exec.Cmd
	socketPath string
	stdin      io.WriteCloser
	stdout     io.ReadCloser
}

// StartVm starts a crosvm instance with the give disk path as an additional disk.
func StartVM(ctx context.Context, dp *string, kernel_args []string) (*RunningVM, error) {
	component_path, err := LoadTerminaComponent(ctx)
	if err != nil {
		return nil, err
	}

	var sp string
	sp, err = GenSocketPath()
	if err != nil {
		return nil, err
	}
	args := []string{"run", "--socket", sp, "--root", component_path + "/vm_rootfs.img"}
	if dp != nil {
		disk_args := []string{"--rwdisk", *dp}
		args = append(args, disk_args...)
	}
	args = append(args, kernel_args...)
	args = append(args, component_path+"/vm_kernel")

	c := exec.Command("crosvm", args...)
	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = c.Start()
	if err != nil {
		return nil, err
	}

	r := &RunningVM{
		ctx:        ctx,
		crosVM:     c,
		socketPath: sp,
		stdin:      stdin,
		stdout:     stdout,
	}
	return r, nil
}

// StopVM stops a VM that was returned from StartVM
func (r *RunningVM) StopVm() error {
	err := exec.Command("crosvm", "stop", r.socketPath).Run()
	if err != nil {
		testing.ContextLogf(r.ctx, "failed to exec stop")
		return err
	}
	err = r.crosVM.Wait()
	if err != nil {
		return err
	}
	return nil
}

// ExecCommand starts a command in the VM by sending it to stdin.
func (r *RunningVM) ExecCommand(c string) error {
	_, err := r.stdin.Write([]byte(c))
	if err != nil {
		testing.ContextLogf(r.ctx, "failed to write command", err)
		return err
	}
	_, err = r.stdin.Write([]byte{'\n'})
	if err != nil {
		testing.ContextLogf(r.ctx, "failed to send newline", err)
		return err
	}
	return nil
}

// Wait for the command prompt to be returned by the VM.
func (r *RunningVM) WaitPrompt(ctx context.Context) (bool, *bytes.Buffer, error) {
	var output bytes.Buffer
	tee := io.TeeReader(r.stdout, &output)
	matched, err := regexp.MatchReader("localhost.+#", bufio.NewReader(io.LimitReader(tee, 16384)))
	if err != nil {
		return false, nil, err
	}
	return matched, &output, nil
}

// Runs the given command and waits for the shell prompt to be returned
func (r *RunningVM) RunCommand(ctx context.Context, cmd string) (*bytes.Buffer, error) {
	err := r.ExecCommand(cmd)
	if err != nil {
		testing.ContextLog(ctx, "Run command failed: ", cmd)
		return nil, err
	}

	testing.ContextLog(ctx, "Ran command: ", cmd)

	var got_prompt bool
	var cmd_output *bytes.Buffer
	got_prompt, cmd_output, err = r.WaitPrompt(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Run command completion failed: ", cmd)
		return nil, err
	}
	if !got_prompt {
		return nil, fmt.Errorf("Failed to find command prompt after: ", cmd)
	}
	return cmd_output, nil
}

func GenSocketPath() (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "crosvm_socket")
	if err != nil {
		return "", err
	}
	name := file.Name()
	os.Remove(name)
	return name, nil
}
