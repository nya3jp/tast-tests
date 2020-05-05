// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dualshock contains the functions that are shared between al
// dualshock controllers.
package dualshock

import (
	"bytes"
	"context"
	"encoding/binary"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/uhid"
)

// Uniq is a hardcoded uniq field for gamepads
const Uniq = "01:23:45:67:89:AB"

// HandleGetReport handles a get report request by the kernel using the byte
// array returned by the processRNum function.
func HandleGetReport(ctx context.Context, processRNum func(uhid.RNumType) ([]byte, error), d *uhid.Device, buf []byte) error {
	reader := bytes.NewReader(buf)
	event := uhid.GetReportRequest{}
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		return err
	}
	var data []byte
	var err error
	if data, err = processRNum(event.RNum); err != nil {
		return errors.Wrap(err, "failed parsing rnum in get report request")
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
