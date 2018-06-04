package dbusutil

import (
	"fmt"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

// CallProtoMethod marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name.
func CallProtoMethod(obj dbus.BusObject, method string, in, out proto.Message) error {
	marshIn, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed marshaling %s arg: %v", method, err)
	}
	var marshOut []byte
	if err = obj.Call(method, 0, marshIn).Store(&marshOut); err != nil {
		return fmt.Errorf("failed calling %s: %v", method, err)
	}
	if err = proto.Unmarshal(marshOut, out); err != nil {
		return fmt.Errorf("failed unmarshaling %s response: %v", method, err)
	}
	return nil
}
