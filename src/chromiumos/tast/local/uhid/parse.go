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
	info := regex.FindStringSubmatch(line)[1:]
	if len(info) != 3 {
		return errors.Errorf("missing parameters in info line: got %d, wanted 3. %+v", len(info), info)
	}

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
	// 0th element of the array is the match of the whole string, we
	// ignore that.
	times := regex.FindStringSubmatch(timeStamp)[1:]
	if len(times) != 2 {
		return time.Until(time.Now()), errors.New("event timestamp must be of the form <seconds>.<microseconds>")
	}

	for i, v := range []*uint64{&seconds, &microSeconds} {
		*v, err = strconv.ParseUint(times[i], 10, 32)
		if err != nil {
			return time.Until(time.Now()), errors.Wrap(err, "failed parsing timestamp")
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
