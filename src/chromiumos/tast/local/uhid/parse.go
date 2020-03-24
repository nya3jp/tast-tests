// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE d.File.

package uhid

import (
	"strconv"
	"strings"
	"time"
)

// parseInfo receives a string containing a bus, vendor id and product
// id and assigns them to the given DeviceData. The string should be
// in the format "<bus> <vendorID> <productID>" where each number is
// separated by a white space.
func parseInfo(data *DeviceData, line string) error {
	var err error
	var bus uint64
	var vendorID uint64
	var productID uint64
	nextWhiteSpace := strings.Index(line, " ")
	if bus, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 16); err != nil {
		return err
	}
	line = line[nextWhiteSpace+1:]
	nextWhiteSpace = strings.Index(line, " ")
	if vendorID, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 32); err != nil {
		return err
	}
	if productID, err = strconv.ParseUint(line[nextWhiteSpace+1:], 16, 32); err != nil {
		return err
	}
	data.Bus = uint16(bus)
	data.VendorID = uint32(vendorID)
	data.ProductID = uint32(productID)
	return nil
}

// parseDescriptor receives a line containing the descriptor and its
// size and then assigns it to data.Descriptor. The string must be of
// the form "<size> <d_0> <d_1> ... <d_size>" where d_i is a
// descriptor element separated from the others by whitespace at
// both sides.
func parseDescriptor(data *DeviceData, line string) error {
	nextWhiteSpace := strings.Index(line, " ")
	size, err := strconv.ParseInt(line[:nextWhiteSpace], 10, 0)
	if err != nil {
		return err
	}
	for i := 0; i < int(size); i++ {
		line = line[nextWhiteSpace+1:]
		var n uint64
		if n, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 8); err != nil {
			return err
		}
		data.Descriptor[i] = byte(n)
		nextWhiteSpace = strings.Index(line, " ")
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
	if seconds, err = strconv.ParseUint(line[:6], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	if microSeconds, err = strconv.ParseUint(line[7:13], 10, 32); err != nil {
		return time.Until(time.Now()), err
	}
	return time.Duration(seconds)*time.Second + time.Duration(microSeconds)*time.Microsecond, nil
}

// parseData returns the device event information given in line. The
// returned []byte can then be passed to d.InjectEvent where d is the
// UHIDDevice created based on this recording. The line received must
// be of the form "<size> <d_1> <d_2> ... <d_size>" where size is the
// size of the returned array and d_i corresponds to element i in the
// returned array, separated by whitespace from the others ath both
// sides.
func parseData(line string) ([]byte, error) {
	nextWhiteSpace := strings.Index(line, " ")
	var size uint64
	var err error
	if size, err = strconv.ParseUint(line[:nextWhiteSpace], 10, 0); err != nil {
		return nil, err
	}
	data := make([]byte, size)
	for i := 0; i < int(size); i++ {
		line = line[nextWhiteSpace+1:]
		var n uint64
		if n, err = strconv.ParseUint(line[:nextWhiteSpace], 16, 8); err != nil {
			return nil, err
		}
		data[i] = byte(n)
		nextWhiteSpace = strings.Index(line, " ")
	}
	return data, nil
}
