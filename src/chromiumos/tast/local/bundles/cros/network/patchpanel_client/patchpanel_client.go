// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package patchpanel interacts with the patchpanel system daemon.
package patchpanel

import (
	"context"
	"os"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	pp "chromiumos/system_api/patchpanel_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

const (
	jobName       = "patchpanel"
	dbusName      = "org.chromium.PatchPanel"
	dbusPath      = "/org/chromium/PatchPanel"
	dbusInterface = "org.chromium.PatchPanel"
)

// Client is a wrapper around patchpanel DBus API.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// New connects to the patchpanel daemon via D-Bus and returns a patchpanel object.
func New(ctx context.Context) (*Client, error) {
	if err := upstart.EnsureJobRunning(ctx, jobName); err != nil {
		return nil, err
	}

	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Client{conn, obj}, nil
}

// ConnectNamespace sends a ConnectNamespaceRequest for the given namespace pid. Returns a pair with
// a valid ScopedFD and the ConnectNamespaceResponse proto message received if the request succeeded.
// Closing the ScopedFD will teardown the veth and routing setup and free the allocated IPv4 subnet.
func (pc *Client) ConnectNamespace(ctx context.Context, pid int32, outboundPhysicalDevice string,
	forwardUserTraffic bool) (*os.File, *pp.ConnectNamespaceResponse, error) {
	method := dbusInterface + "." + "ConnectNamespace"

	request := &pp.ConnectNamespaceRequest{
		Pid:                    pid,
		OutboundPhysicalDevice: outbound_physical_device,
		AllowUserTraffic:       forward_user_traffic,
	}
	response := &pp.ConnectNamespaceResponse{}

	marshRequest, err := proto.Marshal(request)
	if err != nil {
		return nil, response, errors.Wrapf(err, "failed marshaling %s request", method)
	}

	local, remote, err := os.Pipe()
	if err != nil {
		return nil, response, errors.Wrap(err, "failed to open pipe")

	}
	remoteFd := dbus.UnixFD(remote.Fd())
	defer remote.Close()

	var marshResponse []byte
	if err := pc.obj.CallWithContext(ctx, method, 0, marshRequest, remoteFd).Store(&marshResponse); err != nil {
		local.Close()
		return nil, response, errors.Wrapf(err, "failed reading %s response", method)
	}

	if err := proto.Unmarshal(marshResponse, response); err != nil {
		local.Close()
		return nil, response, errors.Wrapf(err, "failed unmarshaling %s response", method)
	}

	return local, response, nil
}
