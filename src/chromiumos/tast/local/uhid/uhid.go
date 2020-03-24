// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE d.File.

// Package uhid supports creating, handling and destroying devices created
// via /dev/uhid
package uhid

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

const (
	// hidMaxDescriptorSize defined here:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=67?q=uhid.h&ss=chromiumos
	// hidMaxDescriptorSize represents the maximum length of a
	// descriptor or an event injected.
	hidMaxDescriptorSize = 4096

	// uhidDestroy and uhidInput2 defined here:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=26?q=uhid.h&ss=chromiumos
	// They represent the request type that will be used for destroying devices
	// and injecting events
	uhidDestroy uint32 = 1
	uhidInput2  uint32 = 12
)

// deviceData encapsulates the non-trivial data that will then be
// copied over to a create request or be used to get information from
// the device. The fixed size byte arrays are meant to replicate this
// struct:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
type deviceData struct {
	Name       [128]byte
	Phys       [64]byte
	Uniq       [64]byte
	Descriptor [hidMaxDescriptorSize]byte
	Bus        uint16
	VendorId   uint32
	ProductId  uint32
}

// UHIDDevice is the main data structure with which a user of this
// package will
type UHIDDevice struct {
	Data        deviceData
	HidrawNodes []string
	EventNodes  []string
	File        *os.File
}

// struct attempts to replicate C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
// an input request is used to inject events into the created device.
type uhidInput2Request struct {
	RequestType uint32
	DataSize    uint16
	Data        [hidMaxDescriptorSize]uint8
}

// Creates a device with the attributes specified in d.
func (d *UHIDDevice) NewKernelDevice(ctx context.Context) error {
	var err error
	if d.File, err = os.OpenFile("/dev/uhid", os.O_RDWR, 0644); err != nil {
		return err
	}
	uniq, _ := uuid.NewRandom()
	copy(d.Data.Uniq[:], uniq[:])
	if err = writeEvent(d.File, createRequest(d.Data)); err != nil {
		return err
	}
	if err = receiveuhidStart(d); err != nil {
		return err
	}
	err = deviceNodes(ctx, d)
	return err
}

// destroys the device specified in d by writing a destroy request
// to /dev/uhid the file as well as the hidraw and event nodes are
// cleared.
func (d *UHIDDevice) Destroy() error {
	if d == nil || d.File == nil {
		return errors.New("This device has not been initialized")
	}
	if err := writeEvent(d.File, uhidDestroy); err != nil {
		return err
	}
	if err := d.File.Close(); err != nil {
		return err
	}
	d.File = nil
	d.HidrawNodes = nil
	d.EventNodes = nil
	return nil
}

// Injects an event into an existing device. The data array will vary
// from device to device.
func (d *UHIDDevice) InjectEvent(data []uint8) error {
	if d == nil || d.File == nil {
		return fmt.Errorf("the device given (%p) has not been initialized", d.Data.Name)
	}
	req := uhidInput2Request{}
	req.RequestType = uhidInput2
	req.DataSize = uint16(len(data))
	copy(req.Data[:len(data)], data)
	err := writeEvent(d.File, req)
	return err
}
