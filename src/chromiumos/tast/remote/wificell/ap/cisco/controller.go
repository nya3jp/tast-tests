// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cisco

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Controller is the handle object for wrapper of the Cisco wireless controller device.
type Controller struct {
	stdin         io.WriteCloser
	stdoutScanner *bufio.Scanner
	cmd           *ssh.Cmd
	lines         chan string
}

const (
	promptText   = "(Cisco Controller)"
	loginTimeout = 10 * time.Second
	cmdTimeout   = 10 * time.Second
)

// InitCiscoController creates Controller, connects and logs in to the controller device.
func InitCiscoController(ctx context.Context, viaConn *ssh.Conn, hostname, user, password string) (*Controller, error) {
	ctx, st := timing.Start(ctx, "ConnectCiscoController")
	defer st.End()

	var ctrl Controller

	err := ctrl.openConnection(ctx, hostname, viaConn)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open connection to Cisco controller %s", hostname)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	// wait for initial prompt line
	if _, err := ctrl.waitCompleteResult(timeoutCtx); err != nil {
		return nil, errors.Wrap(err, "failed waiting for Cisco controller login prompt")
	}
	if _, err := io.WriteString(ctrl.stdin, user+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to send login to Cisco controller")
	}
	if _, err := io.WriteString(ctrl.stdin, password+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to send password to Cisco controller")
	}

	if _, err := ctrl.waitCompleteResult(timeoutCtx); err != nil {
		return nil, errors.Wrap(err, "failed waiting for Cisco controller command prompt")
	}

	out, err := ctrl.sendCommand(ctx, "show wlan summary")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get command result")
	}
	testing.ContextLog(ctx, "Test command result: ", out)

	// TODO check and remove any WLANs

	return &ctrl, nil
}

func (ctrl *Controller) openConnection(ctx context.Context, hostname string, dutConn *ssh.Conn) error {
	cmd := dutConn.Command("sudo", "ssh", hostname)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe to controller console")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe from  controller console")
	}

	if err := cmd.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start wpa_cli")
	}

	ctrl.lines = make(chan string, 100)

	ctrl.stdin = stdin
	ctrl.stdoutScanner = bufio.NewScanner(stdout)
	ctrl.stdoutScanner.Split(scanPrompt)
	ctrl.cmd = cmd

	go func() {
		defer close(ctrl.lines)
		for ctrl.stdoutScanner.Scan() {
			line := ctrl.stdoutScanner.Text()
			ctrl.lines <- line
		}
	}()

	return nil
}

// scanPrompt is a split function for a Scanner that returns each command
// with its result.
// Command prompt text "(Cisco Controller)" is used as a separator of commands,
// it is stripped from the result. The prompt char ">" is left to indicate
// start of command.
// The last non-empty line of input will be returned even if it does not end
// with command prompt.
func scanPrompt(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte(promptText)); i >= 0 {
		// advance over the prompt and trailing spaces
		skip := len(promptText)
		for data[i+skip] == ' ' && i+skip < len(data) {
			skip = skip + 1
		}
		// We have a full prompt-terminated buffer
		return i + skip, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated result. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func (ctrl *Controller) waitCompleteResult(ctx context.Context) (result string, err error) {
	for {
		select {
		case <-ctx.Done():
			testing.ContextLog(ctx, "timeout")
			return "", errors.New("Timeout while waiting for prompt")
		case line := <-ctrl.lines:
			if len(line) == 0 {
				continue
			}
			if line[0] == '>' {
				// remove leading prompt and command
				newLineIdx := strings.IndexRune(line, '\n')
				if newLineIdx != -1 {
					return line[newLineIdx+1:], nil
				}
			}
			return line, nil
		}
	}
	testing.ContextLog(ctx, "ret?")
	return "", nil
}

func (ctrl *Controller) sendCommand(ctx context.Context, cmd string) (output string, err error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
	defer cancel()
	if _, err := io.WriteString(ctrl.stdin, cmd+"\n"); err != nil {
		testing.ContextLog(ctx, "Failed to send command to Cisco controller: ", err)
	}

	return ctrl.waitCompleteResult(timeoutCtx)
}
