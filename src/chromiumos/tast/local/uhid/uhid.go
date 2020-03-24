// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uhid supports creating, handling and destroying devices created
// via /dev/uhid
package uhid

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
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

	// uhidEventSize refers to the size of this C struct:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=179?q=uhid.h&ss=chromiumos
	// This is the struct that is always written by the kernel to
	// /dev/uhid
	uhidEventSize = 4380

	// the following constants defined here:
	// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=26?q=uhid.h&ss=chromiumos
	// These constants are used for event handlers. If the user wishes
	// to handle an event UHIDEvent then the corresponding handler must
	// be set in the EventHandlers map.

	// Destroy destroys the device
	Destroy uint32 = 1
	// Start is written by the kernel to acknowledge the creation of
	// a device
	Start uint32 = 2
	// Stop is written by the kernel to acknowledge the destruction
	// of a device
	Stop uint32 = 3
	// Open is written by the kernel to signal that the data being
	// provided by the device is being read
	Open uint32 = 4
	// Close is written by the kernel to signal that no more processes
	// are reading this device's data
	Close uint32 = 5
	// Output is written by the kernel to signal that the HID device
	// driver wants to send raw data to the I/O device on the interrupt
	// channel
	Output uint32 = 6
	// GetReport is written by the kernel to signal that the kernel
	// driver wants to perform a GET_REPORT request on the control
	// channeld as described in the HID specs
	GetReport uint32 = 9
	// GetReportReply must be written by the user as a reply to a
	// UHIDGetReport request
	GetReportReply uint32 = 10
	// Input2 is used to inject events to the device
	Input2 uint32 = 12
	// SetReport is written by the kernel to signal that the kernel
	// driver wants to perform a SET_REPORT request on the control
	// channeld as described in the HID specs
	SetReport uint32 = 13
	// SetReportReply must be written by the user as a reply to a
	// SetReport request
	SetReportReply uint32 = 14
)

// GetReportRequest can be used by users of this library to handle report
// requests by the kernel.
type GetReportRequest struct {
	RequestType uint32
	ID          uint32
	RNum        uint8
	RType       uint8
}

// GetReportReplyRequest can be used by users of this library to reply to
// report requests by the kernel.
type GetReportReplyRequest struct {
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

type eventHandler func(d *Device, buf []byte) error

// Device is the main data structure with which a user of this
// package will
type Device struct {
	Data        DeviceData
	hidrawNodes []string
	eventNodes  []string
	File        *os.File

	// EventHandlers is used on a call to Dispatch to call the
	// corresponding handling function. If the user wishes to handle a
	// particular event then they must assign their handler function to
	// EventHandlers[UHIDEvent] where UHIDEvent is one of the UHID
	// constants defined above.
	EventHandlers map[uint32]eventHandler
}

// Input2Request attempts to replicate C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
// an input request is used to inject events into the created device.
type Input2Request struct {
	RequestType uint32
	DataSize    uint16
	Data        [hidMaxDescriptorSize]uint8
}

// NewKernelDevice creates a device with the attributes specified in
// d. Only after calling this function will the device be ready for
// the other operations.
func (d *Device) NewKernelDevice(ctx context.Context) error {
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
	d.setDefaultHandlers()
	return d.Dispatch(ctx)
}

// Destroy destroys the device specified in d by writing a destroy
// request to /dev/uhid the file as well as the hidraw and event nodes
// are cleared.
func (d *Device) Destroy() error {
	if d == nil || d.File == nil {
		return errors.New(UninitializedDeviceError)
	}
	if err := d.WriteEvent(Destroy); err != nil {
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
		return errors.New(UninitializedDeviceError)
	}
	req := Input2Request{}
	req.RequestType = Input2
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
	if d == nil || d.File == nil {
		return errors.New(UninitializedDeviceError)
	}
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

// ReadEvent returns a buffer with information read from the given
// device's file. All events arriving to /dev/uhid will be of the
// form of this struct:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=180
// which has a size of uhidEventSize.
func (d *Device) ReadEvent() ([]byte, error) {
	if d == nil || d.File == nil {
		return nil, errors.New(UninitializedDeviceError)
	}

	buf := make([]byte, uhidEventSize)
	n, err := d.File.Read(buf)
	if err != nil {
		return buf, err
	}
	if n != uhidEventSize {
		return buf, errors.Errorf("bytes read: %d, bytes Expected: %d. the ammount of bytes read does not represent a uhid event", n, uhidEventSize)
	}
	return buf, nil
}

// WriteEvent will write the struct given in i into /dev/uhid and
// return an error if unsuccessful.
func (d *Device) WriteEvent(i interface{}) error {
	if d == nil || d.File == nil {
		return errors.New(UninitializedDeviceError)
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, i)
	if err != nil {
		return err
	}
	_, err = d.File.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// Dispatch must ba called when an event needs to be handled. Be sure
// to implement some method of checking if the event you wish to
// handle was indeed the one handled.
func (d *Device) Dispatch(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if d == nil || d.File == nil {
			return errors.New(UninitializedDeviceError)
		}

		var buf []byte
		var err error
		if buf, err = d.ReadEvent(); err != nil {
			return err
		}

		reader := bytes.NewReader(buf[:4]) // We just want to read the first uint32 for now
		var requestType uint32
		if err = binary.Read(reader, binary.LittleEndian, &requestType); err != nil {
			return err
		}
		return d.processEvent(buf, requestType)
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return err
	}
	return nil
}

func (d *Device) processEvent(buf []byte, requestType uint32) error {
	if f := d.EventHandlers[requestType]; f != nil {
		return f(d, buf)
	}
	return errors.Errorf("unknown event: %d", requestType)
}

func (d *Device) setDefaultHandlers() {
	d.EventHandlers = map[uint32]eventHandler{
		Start:     defaultHandler,
		Stop:      defaultHandler,
		Open:      defaultHandler,
		Close:     defaultHandler,
		Output:    defaultHandler,
		GetReport: defaultHandler,
		SetReport: defaultHandler,
	}
}

func defaultHandler(d *Device, buf []byte) error {
	return nil
}

// func (d *Device) processEvent(buf []byte, requestType uint32) error {
// 	if requestType == UHIDStart {
// 		return d.OnStart(d, buf)
// 	} else if requestType == UHIDStop {
// 		return d.OnStop(d, buf)
// 	} else if requestType == UHIDOpen {
// 		return d.OnOpen(d, buf)
// 	} else if requestType == UHIDClose {
// 		return d.OnClose(d, buf)
// 	} else if requestType == UHIDOutput {
// 		return d.OnOutput(d, buf)
// 	} else if requestType == UHIDGetReport {
// 		return d.OnGetReport(d, buf)
// 	} else if requestType == UHIDSetReport {
// 		return d.OnSetReport()
// 	}
// 	return errors.New("Unknown event")
// }
