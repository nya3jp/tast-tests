package uhid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
)

const (
	UHIDEventSize = 4380
)

func readEvent(d *UHIDDevice) ([]byte, error) {
	buf := make([]byte, UHIDEventSize)
	n, err := d.File.Read(buf)
	if err != nil {
		return buf, err
	}
	if n != UHIDEventSize {
		return buf, errors.New("Error waiting for UHID_START")
	}
	return buf, nil
}

func writeEvent(file *os.File, i interface{}) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, i)
	if err != nil {
		return err
	}
	_, err = file.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}
