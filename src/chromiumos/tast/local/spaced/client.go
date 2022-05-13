// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package spaced contains utilities for communicating with spaced.
package spaced

import (
	"context"

	"github.com/godbus/dbus/v5"

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

// NewClient connects to spaced via D-Bus and returns a Client object.
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

// FreeDiskSpace fetches the free space available for a given path.
func (c *Client) FreeDiskSpace(ctx context.Context, path string) (int64, error) {
	var result int64
	if err := c.call(ctx, "GetFreeDiskSpace", path).Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetFreeDiskSpace")
	}
	return result, nil
}

// TotalDiskSpace fetches the total disk space for a given path.
func (c *Client) TotalDiskSpace(ctx context.Context, path string) (int64, error) {
	var result int64
	if err := c.call(ctx, "GetTotalDiskSpace", path).Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetTotalDiskSpace")
	}
	return result, nil
}

// RootDeviceSize fetches the root storage device size for the device.
func (c *Client) RootDeviceSize(ctx context.Context) (int64, error) {
	var result int64
	if err := c.call(ctx, "GetRootDeviceSize").Store(&result); err != nil {
		return 0, errors.Wrap(err, "failed to call method GetRootDeviceSize")
	}
	return result, nil
}
