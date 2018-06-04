package dbusutil

import (
	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

// StoreProtoResponse reads a single byte array argument from call and unmarshals it
// into protobuf, a protocol buffer struct.
func StoreProtoResponse(call *dbus.Call, pb proto.Message) error {
	var marsh []byte
	if err := call.Store(&marsh); err != nil {
		return err
	}
	return proto.Unmarshal(marsh, pb)
}
