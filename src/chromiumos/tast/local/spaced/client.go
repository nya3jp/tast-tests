// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spaced

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.Spaced"
	dbusPath      = "/org/chromium/Spaced"
	dbusInterface = "org.chromium.Spaced"
)

// Client is used to interact with the spaced process over D-Bus.
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

// call is a thin wrapper over CallWithContext.
func (c *Client) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// GetFreeDiskSpace fetches the free space available for a given path.
func (c *Client) GetFreeDiskSpace(ctx context.Context, path string) (uint64, error) {
	var result uint64
	if err := c.call(ctx, "GetFreeDiskSpace", path).Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetFreeDiskSpace")
	}
	return result, nil
}

// GetTotalDiskSpace fetches the total disk space for a given path.
func (c *Client) GetTotalDiskSpace(ctx context.Context, path string) (uint64, error) {
	var result uint64
	if err := c.call(ctx, "GetTotalDiskSpace", path).Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetTotalDiskSpace")
	}
	return result, nil
}
