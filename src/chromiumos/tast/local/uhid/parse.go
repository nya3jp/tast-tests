// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uhid

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// parseInfo receives a string containing a bus, vendor id and product
// id and assigns them to the given DeviceData. The string should be
// in the format "<bus> <vendorID> <productID>" where each number is
// separated by a white space.
func parseInfo(dd *DeviceData, line string) error {
	var bus uint64
	var vendorID uint64
	var productID uint64

	regex, err := regexp.Compile("[0-9]+")
	if err != nil {
		return err
	}

	info := regex.FindAllString(line, -1)
	if len(info) != 3 {
		return errors.Errorf("missing parameters in info line. expected 3, received %d", len(info))
	}

	if bus, err = strconv.ParseUint(info[0], 16, 16); err != nil {
		return err
	}
	if vendorID, err = strconv.ParseUint(info[1], 16, 32); err != nil {
		return err
	}
	if productID, err = strconv.ParseUint(info[2], 16, 32); err != nil {
		return err
	}

	dd.Bus = uint16(bus)
	dd.VendorID = uint32(vendorID)
	dd.ProductID = uint32(productID)
	return nil
}

// parseDescriptor receives a line containing the descriptor and its
// size and then assigns it to data.Descriptor. The string must be of
// the form "<size> <d_0> <d_1> ... <d_size>" where d_i is a
// descriptor element separated from the others by whitespace at
// both sides.
func parseDescriptor(data *DeviceData, line string) error {
	descriptor := strings.Fields(line)
	size, err := strconv.ParseUint(descriptor[0], 10, 16)
	if err != nil {
		return err
	}
	if size != uint64(len(descriptor[1:])) {
		return errors.New("specified descriptor length does not match actual length")
	}

	for i, v := range descriptor[1:] {
		n, err := strconv.ParseUint(v, 16, 8)
		if err != nil {
			return err
		}
		data.Descriptor[i] = byte(n)
	}
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
	regex, err := regexp.Compile("[0-9]+")
	if err != nil {
		return time.Until(time.Now()), err
	}
	times := regex.FindAllString(timeStamp, -1)
	if len(times) != 2 {
		return time.Until(time.Now()), errors.New("event timestamp must be of the form <seconds>.<microseconds>")
	}
	if seconds, err = strconv.ParseUint(times[0], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	if microSeconds, err = strconv.ParseUint(times[1], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	return time.Duration(seconds)*time.Second + time.Duration(microSeconds)*time.Microsecond, nil
}

// parseData returns the device event information given in line. The
// returned []byte can then be passed to d.InjectEvent where d is the
// Device created based on this recording. The line received must
// be of the form "<size> <d_1> <d_2> ... <d_size>" where size is the
// size of the returned array and d_i corresponds to element i in the
// returned array, separated by whitespace from the others at both
// sides.
func parseData(line string) ([]byte, error) {
	dataFields := strings.Fields(line)
	size, err := strconv.ParseUint(dataFields[0], 10, 16)
	if err != nil {
		return nil, err
	}
	if size != uint64(len(dataFields[1:])) {
		return nil, errors.New("specified event data length does not match actual length")
	}

	data := make([]byte, size)
	for i, v := range dataFields[1:] {
		n, err := strconv.ParseUint(v, 16, 8)
		if err != nil {
			return nil, err
		}
		data[i] = byte(n)
	}

	return data, nil
}
