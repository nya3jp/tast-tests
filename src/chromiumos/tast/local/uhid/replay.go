// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uhid

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	parsingError              = "Invalid recording file. Line: %d"
	multiDeviceRecordingError = "multi device recordings are not supported. Line: %d"
	emptyFieldError           = "%s line is empty. Line: %d"
)

// NewDeviceFromRecording receives a file containing a hid recording recorded using
// hid-tools (https://gitlab.freedesktop.org/libevdev/hid-tools) and
// creates a device based on the information contained in it.
func NewDeviceFromRecording(ctx context.Context, file *os.File) (*Device, error) {
	data := DeviceData{}
	scanner := bufio.NewScanner(file)
	var line string
	// the protocol used in hid recording files can be found here:
	// https://github.com/bentiss/hid-replay/blob/master/src/hid-replay.txt#L49
	for i := 1; scanner.Scan(); line, i = scanner.Text(), i+1 {
		if strings.HasPrefix(line, "D: ") {
			return nil, errors.Errorf(multiDeviceRecordingError, i)
		} else if strings.HasPrefix(line, "N: ") {
			if len(line) < 4 {
				return nil, errors.Errorf(emptyFieldError, "Name", i)
			}
			copy(data.Name[:], line[3:])
		} else if strings.HasPrefix(line, "I: ") {
			if len(line) < 4 {
				return nil, errors.Errorf(emptyFieldError, "Info", i)
			}
			if err := parseInfo(&data, line[3:]); err != nil {
				return nil, errors.Wrapf(err, parsingError, i)
			}
		} else if strings.HasPrefix(line, "P: ") {
			if len(line) < 4 {
				return nil, errors.Errorf(emptyFieldError, "Phys", i)
			}
			copy(data.Phys[:], line[3:])
		} else if strings.HasPrefix(line, "R: ") {
			if len(line) < 4 {
				return nil, errors.Errorf(emptyFieldError, "Descriptor", i)
			}
			descriptor, err := parseArray(line[3:])
			if err != nil {
				return nil, errors.Wrapf(err, parsingError, i)
			}
			copy(data.Descriptor[:], descriptor[:])
		} else if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "E: ") {
			return nil, errors.Errorf("invalid hid recording prefix. Line: %d", i)
		}
	}
	d := &Device{Data: data}
	return d, nil
}

// Replay receives a file containing a hid recording, parses it and
// injects the events into the given device. An error is returned if
// the recording file is invalid.
func (d *Device) Replay(ctx context.Context, file *os.File) error {
	if d == nil || d.File == nil {
		return errors.New(UninitializedDeviceError)
	}
	scanner := bufio.NewScanner(file)
	var line string
	sleep := time.Duration(0)
	for i := 1; scanner.Scan(); line, i = scanner.Text(), i+1 {
		if strings.HasPrefix(line, "D: ") {
			return errors.Errorf(multiDeviceRecordingError, i)
		}
		if !strings.HasPrefix(line, "E: ") {
			continue
		}
		if len(line) < 15 {
			return errors.Errorf("Event line is empty. Line: %d", i)
		}
		line = line[3:]
		var err error
		var nextTimestamp time.Duration
		if nextTimestamp, err = parseTime(line); err != nil {
			return errors.Wrapf(err, parsingError, i)
		}
		testing.Sleep(ctx, nextTimestamp-sleep)
		sleep = nextTimestamp
		// The timestamp always occupies 13 spaces.
		line = line[14:]
		var data []byte
		if data, err = parseArray(line); err != nil {
			return err
		}
		if err = d.InjectEvent(data); err != nil {
			return err
		}
	}
	return nil
}
