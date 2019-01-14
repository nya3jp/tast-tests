package dbusutil

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/errors"
)

// CallProtoMethod is a convienience version of CallProtoMethodN with a single input argument.
func CallProtoMethod(ctx context.Context, obj dbus.BusObject, method string, in, out proto.Message) error {
	return CallProtoMethodN(ctx, obj, method, []proto.Message{in}, out)
}

// CallProtoMethod2 is a convienience version of CallProtoMethodN with two input arguments.
func CallProtoMethod2(ctx context.Context, obj dbus.BusObject, method string, in1, in2, out proto.Message) error {
	return CallProtoMethodN(ctx, obj, method, []proto.Message{in1, in2}, out)
}

// CallProtoMethodN marshals in arguments, passes them as byte array args to method on obj,
// and unmarshals a byte array arg from the response to out. method should be prefixed
// by a D-Bus interface name. Out may be nil. Any nil in arguments are ignored.
func CallProtoMethodN(ctx context.Context, obj dbus.BusObject, method string, in []proto.Message, out proto.Message) error {
	var args []interface{}

	for index, inProto := range in {

		if inProto != nil {
			marshIn, err := proto.Marshal(inProto)
			if err != nil {
				return errors.Wrapf(err, "failed marshaling %s arg at index %d", method, index)
			}
			args = append(args, marshIn)
		}
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
