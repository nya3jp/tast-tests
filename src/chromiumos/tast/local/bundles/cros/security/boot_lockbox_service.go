// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt" // remove?

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	cpb "chromiumos/system_api/bootlockbox_proto"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const (
	// Define the D-Bus constants here.
	dbusName      = "org.chromium.BootLockbox"
	dbusPath      = "/org/chromium/BootLockbox"
	dbusInterface = "org.chromium.BootLockboxInterface"
)

func init() {
    testing.AddService(&testing.Service{
        Register: func(srv *grpc.Server, s *testing.ServiceState) {
            security.RegisterBootLockboxServiceServer(srv, &BootLockboxService{s})
        },
    })
}

// BootLockboxService implements tast.cros.security.BootLockboxService.
type BootLockboxService struct {
    s *testing.ServiceState
}

func (*BootLockboxService) Read(ctx context.Context, request *security.ReadBootLockboxRequest) (*security.ReadBootLockboxResponse, error) {
	// TODO be careful for byte[] vs string when talking to boot lockbox
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to connect to %s", dbusName)

	}

	marshalled, err := proto.Marshal(&security.ReadBootLockboxRequest{request.Key})
	if err != nil {
		return nil, errors.Wrap(err, "Failed marshaling ReadBootLockboxRequest")
	}
	var marshalledResponse []byte
	if err := obj.CallWithContext(ctx, dbusInterface+".ReadBootLockbox", 0, &marshalled).Store(&marshalledResponse); err != nil {
		return nil, errors.Wrapf(err, "Failed to read from boot lockbox (key: %s)", request.Key)
	//} else {
		//s.Logf("Boot lockbox returns %q", state)
	}
	baseReply := new(cpb.BootLockboxBaseReply)
	if err := proto.Unmarshal(marshalledResponse, baseReply); err != nil {
		return nil, errors.Wrap(err, "Failed unmarshaling ReadBootLockboxResponse")
	}
	if baseReply.GetError() != cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_NOT_SET {
		return nil, fmt.Errorf("readBootLockbox returns error %d", baseReply.GetError()) // TODO create a new error
	}

	iface, err := proto.GetExtension(baseReply, cpb.E_ReadBootLockboxReply_Reply)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get ReadBootLockboxReply's extension")
	}
	readReply := iface.(cpb.ReadBootLockboxReply)
	return &security.ReadBootLockboxResponse{readReply.GetData()}, nil
}

func (*BootLockboxService) Store(ctx context.Context, request *security.StoreBootLockboxRequest) (*empty.Empty, error) {
	return new(empty.Empty), nil
}
