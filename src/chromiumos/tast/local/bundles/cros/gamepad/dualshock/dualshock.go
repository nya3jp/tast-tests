// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dualshock contains the functions that are shared between al
// dualshock controllers.
package dualshock

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"chromiumos/tast/local/uhid"
)

// Uniq returns a randomly generated uniq string for a dualshock3. The
// uniq must be composed of a string of 6 colon-separated unsigned 8
// bit hexadecimal integers for the dualshock 3 to properly function.
func Uniq() string {
	var rands [6]string
	for i := 0; i < len(rands); i++ {
		rands[i] = fmt.Sprintf("%02x", rand.Intn(256))
	}
	return strings.Join(rands[:], ":")
}

// GetReport handles get report requests by the kernel using the byte
// array returned by the processRNum function.
func GetReport(processRNum func(*uhid.Device, uint8) ([]byte, error), d *uhid.Device, buf []byte) error {
	reader := bytes.NewReader(buf)
	event := uhid.GetReportRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}
	data, err := processRNum(d, event.RNum)
	if err != nil {
		return err
	}
	reply := uhid.GetReportReplyRequest{
		RequestType: uhid.GetReportReply,
		ID:          event.ID,
		Err:         0,
		DataSize:    uint16(len(data)),
	}
	copy(reply.Data[:], data[:])
	return d.WriteEvent(reply)
}

// StoreUniqInReport assigns the colon separated elements in uniq to
// the indexes from offset onwards in the report array.
func StoreUniqInReport(report []byte, uniq string, offset int) ([]byte, error) {
	for i, v := range strings.Split(uniq, ":") {
		n, err := strconv.ParseUint(v[:2], 16, 8)
		if err != nil {
			return nil, err
		}
		report[i+offset] = uint8(n)
	}
	return report, nil
}
