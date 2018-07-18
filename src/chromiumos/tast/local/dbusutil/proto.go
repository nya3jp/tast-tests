package dbusutil

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

// CallProtoMethod marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name. Either in or out may be nil.
func CallProtoMethod(ctx context.Context, obj dbus.BusObject, method string, in, out proto.Message) error {
	var args []interface{}
	if in != nil {
		marshIn, err := proto.Marshal(in)
		if err != nil {
			return fmt.Errorf("failed marshaling %s arg: %v", method, err)
		}
		args = append(args, marshIn)
	}

	call := obj.CallWithContext(ctx, method, 0, args...)
	if call.Err != nil {
		return fmt.Errorf("failed calling %s: %v", method, call.Err)
	}
	if out != nil {
		var marshOut []byte
		if err := call.Store(&marshOut); err != nil {
			return fmt.Errorf("failed reading %s response: %v", method, err)
		}
		if err := proto.Unmarshal(marshOut, out); err != nil {
			return fmt.Errorf("failed unmarshaling %s response: %v", method, err)
		}
	}
	return nil
}
