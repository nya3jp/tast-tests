package uhid

import (
	"bytes"
	"encoding/binary"
)

const UHIDStart uint32 = 2

type UHIDStartRequest struct {
	RequestType uint32
	DevFlags    uint64
}

func receiveUHIDStart(d *UHIDDevice) error {
	buf := make([]byte, UHIDEventSize)
	_, err := d.File.Read(buf)
	buf, err = readEvent(d)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	startReq := UHIDStartRequest{}
	for err := binary.Read(reader, binary.LittleEndian, &startReq); err != nil; {
		buf, err = readEvent(d)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	return nil
}
