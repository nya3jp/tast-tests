// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	cpb "chromiumos/system_api/bootlockbox_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
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
			security.RegisterBootLockboxServiceServer(srv, &BootLockboxService{s: s})
		},
	})
}

// BootLockboxService implements tast.cros.security.BootLockboxService.
type BootLockboxService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

func (c *BootLockboxService) NewChromeLogin(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr
	return &empty.Empty{}, nil
}

func (c *BootLockboxService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

func (*BootLockboxService) Read(ctx context.Context, request *security.ReadBootLockboxRequest) (*security.ReadBootLockboxResponse, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", dbusName)
	}

	marshalled, err := proto.Marshal(&security.ReadBootLockboxRequest{Key: request.Key})
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling ReadBootLockboxRequest")
	}

	var marshalledResponse []byte
	if err := obj.CallWithContext(ctx, dbusInterface+".ReadBootLockbox", 0, &marshalled).Store(&marshalledResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to read from boot lockbox (key: %s)", request.Key)
	}

	reply := new(cpb.ReadBootLockboxReply)
	if err := proto.Unmarshal(marshalledResponse, reply); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling ReadBootLockboxReply")
	}
	switch reply.GetError() {
	// Ignore normal error and not surface to the caller for now
	case cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_NOT_SET, cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_NVSPACE_UNINITIALIZED, cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_MISSING_KEY:
		return &security.ReadBootLockboxResponse{Value: reply.GetData()}, nil
	default:
		return nil, errors.Errorf("ReadBootLockbox returns error %d", reply.GetError())
	}
}

func (*BootLockboxService) Store(ctx context.Context, request *security.StoreBootLockboxRequest) (*empty.Empty, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", dbusName)
	}

	marshalled, err := proto.Marshal(&security.StoreBootLockboxRequest{
		Key:   request.Key,
		Value: request.Value,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling StoreBootLockboxRequest")
	}

	var marshalledResponse []byte
	if err := obj.CallWithContext(ctx, dbusInterface+".StoreBootLockbox", 0, &marshalled).Store(&marshalledResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to store to boot lockbox (key: %s)", request.Key)
	}

	reply := new(cpb.StoreBootLockboxReply)
	if err := proto.Unmarshal(marshalledResponse, reply); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling StoreBootLockboxReply")
	}
	switch reply.GetError() {
	// Ignore normal error and not surface to the caller for now
	case cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_NOT_SET, cpb.BootLockboxErrorCode_BOOTLOCKBOX_ERROR_NVSPACE_UNINITIALIZED:
		return &empty.Empty{}, nil
	default:
		return nil, errors.Errorf("StoreBootLockbox returns error %d", reply.GetError())
	}
}
