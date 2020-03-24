package uhid

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// uhidEventSize refers to the size of this C struct:
// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=179?q=uhid.h&ss=chromiumos
// This is the struct that is always written by the kernel to
// /dev/uhid
const uhidEventSize = 4380

// readEvent returns a buffer with information read from the given
// device's file. All events arriving to /dev/uhid will be of the
// form of this struct:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=180
// which has a size of uhidEventSize.
func readEvent(d *UHIDDevice) ([]byte, error) {
	buf := make([]byte, uhidEventSize)
	n, err := d.File.Read(buf)
	if err != nil {
		return buf, err
	}
	if n != uhidEventSize {
		return buf, fmt.Errorf("bytes read: %d, bytes Expected: %d. the ammount of bytes read does not represent a uhid event", n, uhidEventSize)
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
