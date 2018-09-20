// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.cras"
	dbusPath      = "/org/chromium/cras"
	dbusInterface = "org.chromium.cras.Control"
)

// Cras is used to interact with the cras process over D-Bus.
// For detailed spec, please find src/third_party/adhd/cras/README.dbus-api.
type Cras struct {
	obj dbus.BusObject
}

func NewCras(ctx context.Context) (*Cras, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("Failed connection to system bus: %v", err)
	}

	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusName)
	if err := dbusutil.WaitForService(ctx, conn, dbusName); err != nil {
		return nil, fmt.Errorf("Failed waiting for Cras service: %v", err)
	}

	obj := conn.Object(dbusName, dbusPath)
	return &Cras{obj}, nil
}

// CrasNode contains the metadata of Node in Cras.
// Currently fields which is actually needed by tests are defined.
type CrasNode struct {
	Active     bool
	IsInput    bool
	DeviceName string
}

// GetNodes calls cras.Control.GetNodes over D-Bus.
func (c *Cras) GetNodes(ctx context.Context) ([]CrasNode, error) {
	call := c.call(ctx, "GetNodes")
	if call.Err != nil {
		return nil, call.Err
	}
	ns := make([]interface{}, len(call.Body))
	for i := range ns {
		ns[i] = make(map[string]interface{})
	}
	if err := call.Store(ns...); err != nil {
		return nil, err
	}

	nodes := make([]CrasNode, len(ns))
	for i, n := range ns {
		mp := n.(map[string]interface{})
		nodes[i] = CrasNode{
			Active:     mp["Active"].(bool),
			IsInput:    mp["IsInput"].(bool),
			DeviceName: mp["DeviceName"].(string),
		}
	}
	return nodes, nil
}

// call is this wrapper of CallWithContext for convenience.
func (c *Cras) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
