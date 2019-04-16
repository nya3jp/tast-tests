package cryptohome

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.Cryptohome"
	dbusPath      = "/org/chromium/Cryptohome"
	dbusInterface = "org.chromium.CryptohomeInterface"
)

// Dbus is used to interact with the cryptohomed process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeInterface.xml
type Dbus struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewDbus connects to cryptohomed via D-Bus and returns a Dbus object.
func NewDbus(ctx context.Context) (*Dbus, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Dbus{conn, obj}, nil
}

// Mount calls the MountEx cryptohomed D-Bus method.
func (d *Dbus) Mount(
	ctx context.Context, accountID string, authReq cpb.AuthorizationRequest,
	mountReq cpb.MountRequest) error {
	marshAccountID, err := proto.Marshal(
		&cpb.AccountIdentifier{
			AccountId: &accountID,
		})
	if err != nil {
		return errors.Wrap(err, "failed marshaling AccountIdentifier")
	}
	marshAuthReq, err := proto.Marshal(&authReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling AuthorizationRequest")
	}
	marshMountReq, err := proto.Marshal(&mountReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling MountRequest")
	}
	call := d.obj.CallWithContext(
		ctx, "org.chromium.CryptohomeInterface.MountEx", 0, marshAccountID,
		marshAuthReq, marshMountReq)
	if call.Err != nil {
		return errors.Wrap(call.Err, "failed calling cryptohomed MountEx")
	}
	var marshMountReply []byte
	if err := call.Store(&marshMountReply); err != nil {
		return errors.Wrap(err, "failed reading BaseReply")
	}
	var mountReply cpb.BaseReply
	if err := proto.Unmarshal(marshMountReply, &mountReply); err != nil {
		return errors.Wrap(err, "failed unmarshaling BaseReply")
	}
	if mountReply.Error != nil {
		return errors.Errorf("MountEx call failed with %s", mountReply.Error)
	}
	return nil
}
