// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"bufio"
	"fmt"
	"io"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// cmdWithPipes is a helper that allows communicating with a testexec.Cmd via
// reading and writing fill lines from stdout/stdin.
type cmdWithPipes struct {
	cmd    *testexec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
	stderr *bufio.Reader
}

func newCmdWithPipes(cmd *testexec.Cmd) (cmdWithPipes, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return cmdWithPipes{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return cmdWithPipes{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return cmdWithPipes{}, err
	}
	if err := cmd.Start(); err != nil {
		return cmdWithPipes{}, errors.Wrap(err, "failed to start")
	}
	return cmdWithPipes{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewReader(stdout),
		stderr: bufio.NewReader(stderr),
	}, nil
}

func (c cmdWithPipes) errorContext() string {
	msg := ""
	if c.cmd.ProcessState == nil {
		// Nobody has called Wait yet, so do that now.
		if err := c.cmd.Kill(); err != nil {
			msg += fmt.Sprintf(", failed to kill process (%s)", err)
		} else if err := c.cmd.Wait(); err != nil {
			msg += fmt.Sprintf(", failed to wait process (%s)", err)
		}
	}
	if c.cmd.ProcessState != nil {
		msg += fmt.Sprintf(", exited with code %d", c.cmd.ProcessState.ExitCode())
	}
	stderr := ""
	for {
		line, err := c.stderr.ReadString('\n')
		stderr += line
		if err == nil {
			continue
		}
		if err != io.EOF {
			msg += fmt.Sprintf(", reading stderr failed (%v)", err)
		}
		break
	}
	if len(stderr) > 0 {
		msg += fmt.Sprintf(", wrote to stderr %q", stderr)
	}
	return msg
}

// writeLine writes the passed line string followed by a '\n' to the Cmd, and
// flushes stdin.
func (c cmdWithPipes) writeLine(line string) error {
	if _, err := c.stdin.WriteString(line); err != nil {
		return errors.New("failed to write line" + c.errorContext())
	}
	if err := c.stdin.WriteByte('\n'); err != nil {
		return errors.New("failed to write LF" + c.errorContext())
	}
	if err := c.stdin.Flush(); err != nil {
		return errors.New("failed to flush" + c.errorContext())
	}
	return nil
}

// readLine reads a full line from the Cmd's stdout, and returns the line
// without the '\n'.
func (c cmdWithPipes) readLine() (string, error) {
	line, err := c.stdout.ReadString('\n')
	if err != nil {
		return "", errors.New("failed to read line" + c.errorContext())
	}
	// Remove the newline.
	return line[:len(line)-1], nil
}

// wait blocks until the Cmd has exited. Returns helpful context containing
// return code and stderr output on failure.
func (c cmdWithPipes) wait() error {
	if err := c.cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to Wait"+c.errorContext())
	}
	return nil
}
