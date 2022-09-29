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

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const charCR = "\r\n"
const rCharCR = "\n\r"
const starfishName = "Starfish"
const starfishPrompt = "Starfish:~$ "
const cmdNotFound = ": command not found"

// Time to wait between command write and response read
const (
	CommandWaitTime = 200 * time.Millisecond
)

var garbageChars = []byte{27, 91, 49, 50, 68, 27, 91, 74}

const maxReadBufferSize = 1024 //bytes

// SerialPort holds data pertaining to the serial interface exposed on Starfish module
type SerialPort struct {
	port serial.Port
	t    time.Duration
}

// Open handles the open of the Starfish port.
func (s *SerialPort) Open(ctx context.Context, devName string) error {
	testing.ContextLog(ctx, "opening port: ", devName)
	baud := 115200
	readTimeout := 1 * time.Second
	o := serial.NewConnectedPortOpener(devName, baud, readTimeout)
	p, err := o.OpenPort(ctx)
	if err != nil {
		return errors.Wrap(err, "open failed")
	}
	s.port = p
	s.t = CommandWaitTime
	testing.Sleep(ctx, s.t*time.Millisecond)
	return nil
}

// Read handles raw read from the Starfish port.
func (s *SerialPort) Read(ctx context.Context, forgiving bool) (string, error) {
	testing.ContextLog(ctx, "reading from port")
	buf := make([]byte, maxReadBufferSize)
	_, err := s.port.Read(ctx, buf)
	if err != nil {
		if !forgiving {
			return "", errors.Wrap(err, "read failed")
		}
		testing.ContextLog(ctx, "read failure ignored")
		return "", nil
	}
	buf = bytes.Trim(buf, "\x00")
	output := string(buf)
	output = strings.ReplaceAll(output, string(garbageChars), "")
	testing.Sleep(ctx, s.t*time.Millisecond)
	return output, err
}

// Write handles raw write to the Starfish port.
func (s *SerialPort) Write(ctx context.Context, msg string) error {
	testing.ContextLog(ctx, "writing to port")
	buf := []byte(msg)
	x := len(buf)
	n, err := s.port.Write(ctx, buf)
	if err != nil {
		return errors.Wrap(err, "write failed")
	}
	if n != x {
		return errors.Errorf("write length mismatch. expected: %d, actual: %d", n, x)
	}
	testing.Sleep(ctx, s.t*time.Millisecond)
	return err
}

// Flush handles the flush operation on the Starfish output buffer.
func (s *SerialPort) Flush(ctx context.Context) error {
	testing.ContextLog(ctx, "flushing port")
	err := s.port.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush failed")
	}
	testing.Sleep(ctx, s.t*time.Millisecond)
	return nil
}

// Close handles the close of the Starfish port.
func (s *SerialPort) Close(ctx context.Context) error {
	testing.ContextLog(ctx, "closing port")
	err := s.port.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close failed")
	}
	testing.Sleep(ctx, s.t*time.Millisecond)
	return nil
}

// SendCommand handles sending a command and parsing the response
func (s *SerialPort) SendCommand(ctx context.Context, command string) (string, error) {
	testing.ContextLog(ctx, ">>>>>>", command)
	command += charCR
	err := s.Write(ctx, command)
	if err != nil {
		return "", errors.Wrapf(err, "writing command %s failed", command)
	}
	testing.Sleep(ctx, s.t*time.Millisecond)
	response, err := s.Read(ctx, false)
	if err != nil {
		return "", errors.Wrapf(err, "reading response for command %s failed", command)
	}
	response = strings.TrimPrefix(response, command)
	response = strings.ReplaceAll(response, rCharCR, charCR)
	response = strings.ReplaceAll(response, starfishPrompt, "")
	response = strings.TrimSuffix(response, charCR)
	if strings.Contains(response, cmdNotFound) {
		return "", errors.Wrap(err, response)
	}
	testing.ContextLog(ctx, "<<<<<<", response)
	return response, nil
}

// FindStarfish checks if a Starfish module is connected
func FindStarfish(ctx context.Context) (string, error) {
	testing.ContextLog(ctx, "finding starfish")
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
			testing.ContextLog(ctx, "Found starfish: ", dev)
			return dev, nil
		}
	}
	return "", errors.New("starfish not found")
}
