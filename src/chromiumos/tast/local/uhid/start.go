// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE d.File.

package uhid

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// uhidStartRequest attempts to replicate C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=64
type uhidStartRequest struct {
	RequestType uint32
	DevFlags    uint64
}

// receiveUHIDStart waits for the kernel to write a struct of the
// form uhidStartRequest into /dev/uhid, which happens upon successful
// creation of a device.
func receiveUHIDStart(d *UHIDDevice) error {
	buf := make([]byte, uhidEventSize)
	buf, err := d.ReadEvent()
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	startReq := uhidStartRequest{}
	err = binary.Read(reader, binary.LittleEndian, &startReq)
	// uhidStart defined here:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=29?q=uhid.h&ss=chromiumos
	const uhidStart uint32 = 2
	if startReq.RequestType != uhidStart {
		return errors.New("UHID start event was not received")
	}
	return nil
}
