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
	jobName                = "patchpanel"
	dbusName               = "org.chromium.PatchPanel"
	dbusPath               = "/org/chromium/PatchPanel"
	connectNamespaceMethod = "org.chromium.PatchPanel.ConnectNamespace"
)

// Client is a wrapper around patchpanel DBus API.
type Client struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// New connects to the patchpanel daemon via D-Bus and returns a patchpanel Client object.
// TODO(crbug.com/1135106): Implement missing patchpanel D-Bus API methods.
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
// a open file descriptor and the ConnectNamespaceResponse proto message received if the request succeeded.
// Closing the file descriptor will teardown the veth and routing setup and free the allocated IPv4 subnet.
func (c *Client) ConnectNamespace(ctx context.Context, pid int32, outboundPhysicalDevice string,
	forwardUserTraffic bool) (local *os.File, response *pp.ConnectNamespaceResponse, retErr error) {
	request := &pp.ConnectNamespaceRequest{
		Pid:                    pid,
		OutboundPhysicalDevice: outboundPhysicalDevice,
		AllowUserTraffic:       forwardUserTraffic,
	}
	buf, err := proto.Marshal(request)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed marshaling %s request", connectNamespaceMethod)
	}

	local, remote, err := os.Pipe()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to open pipe for creating %s request arg", connectNamespaceMethod)

	}
	remoteFd := dbus.UnixFD(remote.Fd())
	defer remote.Close()

	defer func() {
		if retErr != nil {
			local.Close()
		}
	}()
	if retErr = c.obj.CallWithContext(ctx, connectNamespaceMethod, 0, buf, remoteFd).Store(&buf); retErr != nil {
		return nil, nil, errors.Wrapf(retErr, "failed reading %s response", connectNamespaceMethod)
	}

	response = &pp.ConnectNamespaceResponse{}
	if retErr = proto.Unmarshal(buf, response); retErr != nil {
		return nil, nil, errors.Wrapf(retErr, "failed unmarshaling %s response", connectNamespaceMethod)
	}

	return local, response, nil
}
