package uhid

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const UHIDStart uint32 = 2

// struct attempts to replicate C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=64
type UHIDStartRequest struct {
	RequestType uint32
	DevFlags    uint64
}

// upon successful creation of a device the kernel will write a struct of the
// form UHIDStartRequest into /dev/uhid. We wait to receive this in order to
// be sure that creation was successful.
func receiveUHIDStart(d *UHIDDevice) error {
	buf := make([]byte, UHIDEventSize)
	buf, err := readEvent(d)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	startReq := UHIDStartRequest{}
	err = binary.Read(reader, binary.LittleEndian, &startReq)
	if startReq.RequestType != UHIDStart {
		return errors.New("UHID start was not received")
	}
	return nil
}
