// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE d.File.

// Package uhid supports creating, handling and destroying devices created
// via /dev/uhid
package uhid

import (
	"errors"
	"os"

	"github.com/google/uuid"
)

const (
	HIDMaxDescriptorSize = 4096

	UHIDDestroy uint32 = 1
)

type DeviceData struct {
	Name       [128]byte
	Phys       [64]byte
	Uniq       [64]byte
	Descriptor [HIDMaxDescriptorSize]byte
	Bus        uint16
	VendorId   uint32
	ProductId  uint32
}

type UHIDDevice struct {
	Data        DeviceData
	HidrawNodes []string
	EventNodes  []string
	File        *os.File
}

// Creates a device with the attributes specified in d.
func (d *UHIDDevice) NewKernelDevice() error {
	var err error
	d.File, err = os.OpenFile("/dev/uhid", os.O_RDWR|os.O_SYNC, 0644)
	if err != nil {
		return err
	}
	uniq, _ := uuid.NewRandom()
	copy(d.Data.Uniq[:], uniq[:])
	err = writeEvent(d.File, createRequest(d.Data))
	if err != nil {
		return err
	}
	err = receiveUHIDStart(d)
	if err != nil {
		return err
	}
	err = getDeviceNodes(d)
	return err
}

// destroys the device specified in d by writing a destroy request
// to /dev/uhid the file as well as the hidraw and event nodes are
// cleared.
func (d *UHIDDevice) Destroy() error {
	if d.File == nil {
		return errors.New("This device has not been initialized")
	}
	err := writeEvent(d.File, UHIDDestroy)
	if err != nil {
		return err
	}
	err = d.File.Close()
	if err != nil {
		return err
	}
	d.File = nil
	d.HidrawNodes = nil
	d.EventNodes = nil
	return nil
}
