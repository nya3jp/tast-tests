// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"bufio"
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// cmdWithPipes is a helper that allows communicating with a testexec.Cmd via
// reading and writing fill lines from stdout/stdin.
type cmdWithPipes struct {
	cmd    *testexec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
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
	if err := cmd.Start(); err != nil {
		return cmdWithPipes{}, errors.Wrap(err, "failed to start")
	}
	return cmdWithPipes{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewReader(stdout),
	}, nil
}

func (c cmdWithPipes) killAndDumpLogs(ctx context.Context) {
	if err := c.cmd.Kill(); err != nil {
		testing.ContextLogf(ctx, "Failed to kill %v %v: %v", c.cmd.Path, c.cmd.Args, err)
		return
	}
	testing.ContextLogf(ctx, "Dumping logs for %v %v", c.cmd.Path, c.cmd.Args)
	if err := c.cmd.Wait(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed to wait for killed command: ", err)
	}
}

// writeLine writes the passed line string followed by a '\n' to the Cmd, and
// flushes stdin.
func (c cmdWithPipes) writeLine(ctx context.Context, line string) error {
	if _, err := c.stdin.WriteString(line); err != nil {
		c.killAndDumpLogs(ctx)
		return errors.Wrap(err, "failed to write line")
	}
	if err := c.stdin.WriteByte('\n'); err != nil {
		c.killAndDumpLogs(ctx)
		return errors.Wrap(err, "failed to write LF")
	}
	if err := c.stdin.Flush(); err != nil {
		c.killAndDumpLogs(ctx)
		return errors.Wrap(err, "failed to flush")
	}
	return nil
}

// readLine reads a full line from the Cmd's stdout, and returns the line
// without the '\n'.
func (c cmdWithPipes) readLine(ctx context.Context) (string, error) {
	line, err := c.stdout.ReadString('\n')
	if err != nil {
		c.killAndDumpLogs(ctx)
		return "", errors.Wrap(err, "failed to read line")
	}
	// Remove the newline.
	return line[:len(line)-1], nil
}
