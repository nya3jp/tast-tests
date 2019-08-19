package dbusutil

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/errors"
)

// CallProtoMethod marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name. Either in or out may be nil.
func CallProtoMethod(ctx context.Context, obj dbus.BusObject, method string, in, out proto.Message) error {
	return CallProtoMethodFd(ctx, obj, method, nil, in, out)
}

// CallProtoMethod marshals in, passes it as a byte array arg to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name. Either in or out may be nil.
func CallProtoMethodFd(ctx context.Context, obj dbus.BusObject, method string, fds []uintptr, in, out proto.Message) error {
	var args []interface{}
	if in != nil {
		marshIn, err := proto.Marshal(in)
		if err != nil {
			return errors.Wrapf(err, "failed marshaling %s arg", method)
		}
		args = append(args, marshIn)
	}

	for _, fd := range fds {
		args = append(args, dbus.UnixFD(fd))
	}

	call := obj.CallWithContext(ctx, method, 0, args...)
	if call.Err != nil {
		return errors.Wrapf(call.Err, "failed calling %s", method)
	}
	if out != nil {
		var marshOut []byte
		if err := call.Store(&marshOut); err != nil {
			return errors.Wrapf(err, "failed reading %s response", method)
		}
		if err := proto.Unmarshal(marshOut, out); err != nil {
			return errors.Wrapf(err, "failed unmarshaling %s response", method)
		}
	}
	return nil
}
