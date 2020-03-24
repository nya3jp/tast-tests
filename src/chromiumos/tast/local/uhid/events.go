package uhid

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
)

const UHIDEventSize = 4380

// readEvent returns a buffer with information read from the given
// device's file. All events arriving to /dev/uhid will be of the
// form of this struct:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=180
// which has a size of UHIDEventSize.
func readEvent(d *UHIDDevice) ([]byte, error) {
	buf := make([]byte, UHIDEventSize)
	n, err := d.File.Read(buf)
	if err != nil {
		return buf, err
	}
	if n != UHIDEventSize {
		return buf, errors.New("Error waiting for event")
	}
	return buf, nil
}

// writeEvent will write the struct given in i into /dev/uhid and
// return an error if unsuccessful.
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
