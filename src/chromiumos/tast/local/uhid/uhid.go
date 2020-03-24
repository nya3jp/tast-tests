// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uhid supports creating, handling and destroying devices created
// via /dev/uhid
package uhid

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// UninitializedDeviceError is returned when the given device is nil
	// or its File is nil (when NewKernelDevice() hasn't been called).
	UninitializedDeviceError  = "device has not been initialized"
	multiDeviceRecordingError = "multi device recordings are not supported"
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

// GetReport can be used by users of this library to handle report
// requests by the kernel.
type GetReport struct {
	RequestType uint32
	ID          uint32
	RNum        uint8
	RType       uint8
}

// GetReportReply can be used by users of this library to reply to
// report requests by the kernel.
type GetReportReply struct {
	RequestType uint32
	ID          uint32
	RNum        uint8
	RType       uint8
	Data        [hidMaxDescriptorSize]byte
}

// DeviceData encapsulates the non-trivial data that will then be
// copied over to a create request or be used to get information from
// the device. The fixed size byte arrays are meant to replicate this
// struct:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
type DeviceData struct {
	Name       [128]byte
	Phys       [64]byte
	Uniq       [64]byte
	Descriptor [hidMaxDescriptorSize]byte
	Bus        uint16
	VendorID   uint32
	ProductID  uint32
}

// Device is the main data structure with which a user of this
// package will
type Device struct {
	Data        DeviceData
	hidrawNodes []string
	eventNodes  []string
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

// NewKernelDevice creates a device with the attributes specified in
// d. Only after calling this function will the device be ready for
// the other operations.
func (d *Device) NewKernelDevice() error {
	if d == nil {
		return errors.New(UninitializedDeviceError)
	}
	var err error
	if d.File, err = os.OpenFile("/dev/uhid", os.O_RDWR, 0644); err != nil {
		return err
	}
	// check if uniq is empty
	if d.Data.Uniq == [64]byte{} {
		uniq, _ := uuid.NewRandom()
		copy(d.Data.Uniq[:], uniq[:])
	}
	if err = d.WriteEvent(createRequest(d.Data)); err != nil {
		return err
	}
	return d.receiveUHIDStart()
}

// Destroy destroys the device specified in d by writing a destroy
// request to /dev/uhid the file as well as the hidraw and event nodes
// are cleared.
func (d *Device) Destroy() error {
	if d == nil || d.File == nil {
		return errors.New(UninitializedDeviceError)
	}
	if err := d.WriteEvent(uhidDestroy); err != nil {
		return err
	}
	if err := d.File.Close(); err != nil {
		return err
	}
	d.File = nil
	d.hidrawNodes = nil
	d.eventNodes = nil
	return nil
}

// InjectEvent Injects an event into an existing device. The data array
// will vary from device to device.
func (d *Device) InjectEvent(data []uint8) error {
	if d == nil || d.File == nil {
		return errors.Errorf("the device given (%s) has not been initialized", d.Data.Name)
	}
	req := uhidInput2Request{}
	req.RequestType = uhidInput2
	req.DataSize = uint16(len(data))
	copy(req.Data[:len(data)], data)
	err := d.WriteEvent(req)
	return err
}

// Recorded receives a file containing a hid recording recorded using
// hid-tools (https://gitlab.freedesktop.org/libevdev/hid-tools) and
// creates a device based on the information contained in it.
func Recorded(ctx context.Context, file *os.File) (*Device, error) {
	data := DeviceData{}
	scanner := bufio.NewScanner(file)
	var line string
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
			if err := parseDescriptor(&data, line[3:]); err != nil {
				return nil, err
			}
		}
	}
	d := &Device{Data: data}
	return d, nil
}

// Replay receives a file containing a hid recording, parses it and
// injects the events into the given device. An error is returned if
// the recording file is invalid.
func (d *Device) Replay(ctx context.Context, file *os.File) error {
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
		// the timestamp always occupies 13 spaces
		line = line[14:]
		var data []byte
		if data, err = parseData(line); err != nil {
			return err
		}
		if err = d.InjectEvent(data); err != nil {
			return err
		}
	}
	return nil
}

// HIDRawNodes returns the /dev/hidraw* paths associated to this
// device. It fetches them if they hadn't yet been fetched.
func (d *Device) HIDRawNodes(ctx context.Context) ([]string, error) {
	if d == nil || d.File == nil {
		return nil, errors.New(UninitializedDeviceError)
	}
	if d.hidrawNodes == nil {
		if err := deviceNodes(ctx, d); err != nil {
			return nil, err
		}
	}
	return d.hidrawNodes, nil
}

// EventNodes returns the /dev/input/event* paths associated to this
// device. It fetches them if they hadn't yet been fetched.
func (d *Device) EventNodes(ctx context.Context) ([]string, error) {
	if d == nil || d.File == nil {
		return nil, errors.New(UninitializedDeviceError)
	}
	if d.eventNodes == nil {
		if err := deviceNodes(ctx, d); err != nil {
			return nil, err
		}
	}
	return d.eventNodes, nil
}
