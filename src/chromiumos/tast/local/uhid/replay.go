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

// Recorded receives a file containing a hid recording recorded using
// hid-tools (https://gitlab.freedesktop.org/libevdev/hid-tools) and
// creates a device based on the information contained in it.
func Recorded(ctx context.Context, file *os.File) (*Device, error) {
	data := DeviceData{}
	scanner := bufio.NewScanner(file)
	var line string
	// the protocol used in hid recording files can be found here:
	// https://github.com/bentiss/hid-replay/blob/master/src/hid-replay.txt#L49
	for ; scanner.Scan(); line = scanner.Text() {
		if strings.HasPrefix(line, "D: ") {
			return nil, errors.New(multiDeviceRecordingError)
		} else if strings.HasPrefix(line, "N: ") {
			copy(data.Name[:], line[3:])
		} else if strings.HasPrefix(line, "I: ") {
			if err := parseInfo(&data, line[3:]); err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(line, "P: ") {
			copy(data.Phys[:], line[3:])
		} else if strings.HasPrefix(line, "R: ") {
			descriptor, err := parseArray(line[3:])
			if err != nil {
				return nil, err
			}
			copy(data.Descriptor[:], descriptor[:])
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
	var sleep time.Duration = time.Duration(0)
	for ; scanner.Scan(); line = scanner.Text() {
		if strings.HasPrefix(line, "D: ") {
			return errors.New(multiDeviceRecordingError)
		}
		if !strings.HasPrefix(line, "E: ") {
			continue
		}
		line = line[3:]
		var err error
		var nextTimestamp time.Duration
		if nextTimestamp, err = parseTime(line); err != nil {
			return err
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
