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

	fserial "chromiumos/tast/common/firmware/serial"
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
const (
	CommandWaitTime = 250 * time.Millisecond
)

var garbageChars = string([]byte{27, 91, 49, 50, 68, 27, 91, 74})

const maxReadBufferSize = 1024 //bytes

// SerialInterface holds data pertaining to the serial interface exposed on Starfish module
type SerialInterface struct {
	port fserial.Port
}

// Open handles the open of the Starfish serial interface.
func (s *SerialInterface) Open(ctx context.Context) error {
	devName, err := FindStarfish(ctx)
	if err != nil {
		return errors.Wrap(err, "find failed")
	}
	//testing.ContextLog(ctx, "opening serial interface")
	baud := 115200
	readTimeout := 1 * time.Second
	o := fserial.NewConnectedPortOpener(devName, baud, readTimeout)
	p, err := o.OpenPort(ctx)
	if err != nil {
		return errors.Wrap(err, "open failed")
	}
	s.port = p
	testing.Sleep(ctx, CommandWaitTime)
	return nil
}

// Read handles raw read from the Starfish serial interface.
func (s *SerialInterface) Read(ctx context.Context) (string, error) {
	//testing.ContextLog(ctx, "reading from serial interface")
	buf := make([]byte, maxReadBufferSize)
	_, err := s.port.Read(ctx, buf)
	if err != nil {
		return "", errors.Wrap(err, "read failed")
	}
	buf = bytes.Trim(buf, "\x00")
	output := string(buf)
	output = strings.ReplaceAll(output, garbageChars, "")
	testing.Sleep(ctx, CommandWaitTime)
	return output, err
}

// Write handles raw write to the Starfish serial interface.
func (s *SerialInterface) Write(ctx context.Context, msg string) error {
	//testing.ContextLog(ctx, "writing to serial interface")
	buf := []byte(msg)
	x := len(buf)
	n, err := s.port.Write(ctx, buf)
	if err != nil {
		return errors.Wrap(err, "write failed")
	}
	if n != x {
		return errors.Errorf("write length mismatch. expected: %d, actual: %d", n, x)
	}
	testing.Sleep(ctx, CommandWaitTime)
	return err
}

// Flush handles the flush operation on the Starfish output buffer.
func (s *SerialInterface) Flush(ctx context.Context) error {
	//testing.ContextLog(ctx, "flushing serial interface")
	err := s.port.Flush(ctx)
	if err != nil {
		return errors.Wrap(err, "flush failed")
	}
	testing.Sleep(ctx, CommandWaitTime)
	return nil
}

// Close handles the close of the Starfish serial interface.
func (s *SerialInterface) Close(ctx context.Context) error {
	//testing.ContextLog(ctx, "closing serial interface")
	err := s.port.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close failed")
	}
	testing.Sleep(ctx, CommandWaitTime)
	return nil
}

// SendCommand handles sending a command and parsing the response
func (s *SerialInterface) SendCommand(ctx context.Context, command string) ([]string, error) {
	//testing.ContextLog(ctx, ">>>>>>", command)
	command += charCR
	err := s.Write(ctx, command)
	if err != nil {
		return nil, errors.Wrapf(err, "writing command %s failed", command)
	}
	testing.Sleep(ctx, 2*CommandWaitTime)
	response, err := s.Read(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "reading response for command %s failed", command)
	}
	responses, cnf := ParseOutput(ctx, response, command)
	/*
		for _, line := range responses {
			testing.ContextLog(ctx, "<<<<<<", line)
		}
	*/
	if cnf != "" {
		return nil, errors.Wrap(err, cnf)
	}
	return responses, nil
}

// FindStarfish checks if a Starfish module is connected
func FindStarfish(ctx context.Context) (string, error) {
	//testing.ContextLog(ctx, "finding starfish")
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

// ParseOutput sanitizes the raw reads from the serial port and puts out a string array
func ParseOutput(ctx context.Context, response, cmd string) ([]string, string) {
	cnf := ""
	response = strings.TrimPrefix(response, cmd)
	response = strings.ReplaceAll(response, rCharCR, charCR)
	response = strings.ReplaceAll(response, starfishPrompt, "")
	response = strings.TrimSuffix(response, charCR)
	i := 0
	lines := strings.Split(response, charCR)
	for _, line := range lines {
		if strings.HasPrefix(line, logPrefix) {
			testing.ContextLog(ctx, "------", line)
		} else {
			lines[i] = line
			i++
			if strings.Contains(line, cmdNotFound) {
				cnf = line
			}
		}
	}
	lines = lines[:i]
	return lines, cnf
}
