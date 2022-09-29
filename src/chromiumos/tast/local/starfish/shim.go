// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package starfish

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const charCR = "\r\n"
const rCharCR = "\n\r"
const starfishName = "Starfish"
const starfishPrompt = starfishName + ":~$ "
const cmdNotFound = ": command not found"
const logPrefix = "<inf> "

// Time to wait between command write and response read
const commandWaitTime = 250 * time.Millisecond
const readTimeout = 1 * time.Second
const baud = 115200

var garbageChars = string([]byte{27, 91, 49, 50, 68, 27, 91, 74})

const maxReadBufferSize = 16384 //bytes

// shim holds data pertaining to the serial interface exposed on Starfish module
type shim struct {
	siface SerialInterface
}

// Open handles the initialization of the Starfish SHIM layer.
func (s *shim) Init(ctx context.Context) ([]string, error) {
	devName, err := findStarfish(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "find failed")
	}
	var sif SerialInterface
	err = sif.Open(ctx, devName, baud, readTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "open failed")
	}
	s.siface = sif
	testing.Sleep(ctx, commandWaitTime)
	err = s.siface.Flush(ctx)
	if err != nil {
		return nil, err
	}
	testing.Sleep(ctx, commandWaitTime)
	_, logs, _ := s.SendCommand(ctx, "")
	return logs, nil
}

// Close handles the close of the Starfish SHIM layer.
func (s *shim) Close(ctx context.Context) error {
	err := s.siface.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close failed")
	}
	return nil
}

// SendCommand handles sending a command and parsing the response
func (s *shim) SendCommand(ctx context.Context, command string) ([]string, []string, error) {
	command += charCR
	inBuf := []byte(command)
	x := len(inBuf)
	n, err := s.siface.Write(ctx, inBuf)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "writing command %s failed", command)
	}
	if n != x {
		return nil, nil, errors.Errorf("write command %s, length mismatch. expected: %d, actual: %d", command, n, x)
	}

	testing.Sleep(ctx, 2*commandWaitTime)

	outBuf := make([]byte, maxReadBufferSize)
	_, err = s.siface.Read(ctx, outBuf)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "reading response for command %s failed", command)
	}
	outBuf = bytes.Trim(outBuf, "\x00")
	response := string(outBuf)
	response = strings.ReplaceAll(response, garbageChars, "")
	responses, cnf, logs := parseOutput(ctx, response, command)
	if cnf != "" {
		return nil, logs, errors.Wrap(err, cnf)
	}
	return responses, logs, nil
}

// findStarfish checks if a Starfish module is connected
func findStarfish(ctx context.Context) (string, error) {
	cmd := testexec.CommandContext(ctx, "find", "/dev/")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var listOfDevs string = string(output)
	re := regexp.MustCompile(`/dev/ttyACM\S*`)
	var ldevs []string = re.FindAllString(listOfDevs, -1)
	for _, dev := range ldevs {
		cmd := testexec.CommandContext(ctx, "udevadm", "info", "--query=symlink", "--name="+dev)
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		if strings.Contains(string(output), starfishName) {
			return dev, nil
		}
	}
	return "", errors.New("starfish not found")
}

// parseOutput sanitizes the raw reads from the serial port and puts out a string array
func parseOutput(ctx context.Context, response, cmd string) ([]string, string, []string) {
	cnf := ""
	var logs []string
	response = strings.TrimPrefix(response, cmd)
	response = strings.ReplaceAll(response, rCharCR, charCR)
	response = strings.ReplaceAll(response, starfishPrompt, "")
	response = strings.TrimSuffix(response, charCR)
	i := 0
	lines := strings.Split(response, charCR)
	for _, line := range lines {
		if strings.HasPrefix(line, logPrefix) {
			logs = append(logs, line)
		} else {
			lines[i] = line
			i++
			if strings.Contains(line, cmdNotFound) {
				cnf = line
			}
		}
	}
	lines = lines[:i]
	return lines, cnf, logs
}
