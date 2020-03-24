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

// uhidStart defined here:
// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h;l=29?q=uhid.h&ss=chromiumos
const uhidStart uint32 = 2

// upon successful creation of a device the kernel will write a struct of the
// form uhidStartRequest into /dev/uhid. We wait to receive this in order to
// be sure that creation was successful.
func receiveuhidStart(d *UHIDDevice) error {
	buf := make([]byte, uhidEventSize)
	buf, err := readEvent(d)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(buf)
	startReq := uhidStartRequest{}
	err = binary.Read(reader, binary.LittleEndian, &startReq)
	if startReq.RequestType != uhidStart {
		return errors.New("UHID start event was not received")
	}
	return nil
}
