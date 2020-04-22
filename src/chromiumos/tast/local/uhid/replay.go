// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uhid

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"
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

// parseInfo receives a string containing a bus, vendor id and product
// id and assigns them to the DeviceData dd.
func parseInfo(dd *DeviceData, line string) error {
	var bus uint64
	var vendorID uint64
	var productID uint64
	var err error

	// The string should be in the format "<bus> <vendorID> <productID>"
	// where each number is separated by a white space.
	regex := regexp.MustCompile(`([0-9a-f]+)\s+([0-9a-f]+)\s+([0-9a-f]+)`)

	// 0th element of the array is the match of the whole string, we
	// ignore that.
	info := regex.FindStringSubmatch(line)
	if len(info) != 4 {
		return errors.Errorf("Info line does not conform to expected format: got %d elements, wanted: 3", len(info)-1)
	}

	// we ignore the first element which is the match corresponding to
	// the whole string.
	info = info[1:]
	for i, v := range []*uint64{&bus, &vendorID, &productID} {
		*v, err = strconv.ParseUint(info[i], 16, 16)
		if err != nil {
			return errors.Wrapf(err, "failed to parse device info item number %d", i+1)
		}
	}

	dd.Bus = uint16(bus)
	dd.VendorID = uint32(vendorID)
	dd.ProductID = uint32(productID)

	return nil
}

// parseTime returns a duration based on the received line that
// represents a time stamp. The line must be of the form
// "<seconds>.<microseconds>" where both seconds and microseconds are
// six digit long decimal numbers separated by a dot.
func parseTime(line string) (time.Duration, error) {
	var seconds uint64
	var microSeconds uint64
	var err error

	timeStamp := strings.Fields(line)[0]
	regex := regexp.MustCompile(`(\d+)\.(\d+)`)
	times := regex.FindStringSubmatch(timeStamp)
	if len(times) != 3 {
		return 0, errors.New("event timestamp must be of the form <seconds>.<microseconds>")
	}

	// we ignore the first element which is the match corresponding to
	// the whole string.
	times = times[1:]
	for i, v := range []*uint64{&seconds, &microSeconds} {
		*v, err = strconv.ParseUint(times[i], 10, 32)
		if err != nil {
			return 0, errors.Wrap(err, "failed parsing timestamp")
		}
	}

	return time.Duration(seconds)*time.Second + time.Duration(microSeconds)*time.Microsecond, nil
}

// parseArray returns the array represented by line as a byte array.
// Line should be of the form "<size> <d_1> <d_2> ... <d_size>" where
// size is the size of the returned array and d_i corresponds to
// element i in the returned array, separated by whitespace from the
// others at both sides.
func parseArray(line string) ([]byte, error) {
	dataFields := strings.Fields(line)
	if len(dataFields) == 0 {
		return nil, errors.New("Empty array for parsing")
	}
	size, err := strconv.ParseUint(dataFields[0], 10, 16)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing event data array length")
	}
	if size != uint64(len(dataFields[1:])) {
		return nil, errors.Errorf("specified event data length does not match actual length; got %d, want %d", len(dataFields[1:]), size)
	}

	data := make([]byte, size)
	for i, v := range dataFields[1:] {
		n, err := strconv.ParseUint(v, 16, 8)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing event data element")
		}
		data[i] = byte(n)
	}

	return data, nil
}
