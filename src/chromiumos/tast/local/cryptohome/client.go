// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// Client is used to interact with the cryptohomed process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeInterface.xml
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewClient connects to cryptohomed via D-Bus and returns a Client object.
func NewClient(ctx context.Context) (*Client, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Client{conn, obj}, nil
}

// Mount calls the MountEx cryptohomed D-Bus method.
func (c *Client) Mount(
	ctx context.Context, accountID string, authReq *cpb.AuthorizationRequest,
	mountReq *cpb.MountRequest) error {
	marshAccountID, err := proto.Marshal(
		&cpb.AccountIdentifier{
			AccountId: &accountID,
		})
	if err != nil {
		return errors.Wrap(err, "failed marshaling AccountIdentifier")
	}
	marshAuthReq, err := proto.Marshal(authReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling AuthorizationRequest")
	}
	marshMountReq, err := proto.Marshal(mountReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling MountRequest")
	}
	call := c.obj.CallWithContext(
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

// CheckKey calls the CheckKeyEx cryptohomed D-Bus method.
func (c *Client) CheckKey(
	ctx context.Context, accountID string, authReq *cpb.AuthorizationRequest) error {
	marshAccountID, err := proto.Marshal(
		&cpb.AccountIdentifier{
			AccountId: &accountID,
		})
	if err != nil {
		return errors.Wrap(err, "failed marshaling AccountIdentifier")
	}
	marshAuthReq, err := proto.Marshal(authReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling AuthorizationRequest")
	}
	marshCheckKeyReq, err := proto.Marshal(&cpb.CheckKeyRequest{})
	if err != nil {
		return errors.Wrap(err, "failed marshaling CheckKeyRequest")
	}
	call := c.obj.CallWithContext(
		ctx, "org.chromium.CryptohomeInterface.CheckKeyEx", 0, marshAccountID,
		marshAuthReq, marshCheckKeyReq)
	if call.Err != nil {
		return errors.Wrap(call.Err, "failed calling cryptohomed CheckKeyEx")
	}
	var marshCheckKeyReply []byte
	if err := call.Store(&marshCheckKeyReply); err != nil {
		return errors.Wrap(err, "failed reading BaseReply")
	}
	var checkKeyReply cpb.BaseReply
	if err := proto.Unmarshal(marshCheckKeyReply, &checkKeyReply); err != nil {
		return errors.Wrap(err, "failed unmarshaling BaseReply")
	}
	if checkKeyReply.Error != nil {
		return errors.Errorf("CheckKeyEx call failed with %s", checkKeyReply.Error)
	}
	return nil
}
