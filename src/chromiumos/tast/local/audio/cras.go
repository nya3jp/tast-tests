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
// Currently fields which are actually needed by tests are defined.
// Please find src/third_party/adhd/cras/README.dbus-api for the meaning of
// each fields.
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

	// cras.Control.GetNodes D-Bus method's signature is not fixed.
	// Specifically, the number of output values depends on the actual
	// number of nodes.
	// That usage is not common practice, and it is less support in
	// godbus. Here, instead, values are manually converted via
	// dbus.Variant.
	nodes := make([]CrasNode, len(call.Body))
	for i, n := range call.Body {
		mp := n.(map[string]dbus.Variant)
		active, ok := mp["Active"]
		if !ok {
			return nil, fmt.Errorf("'Active' not found: %v", mp)
		}
		if nodes[i].Active, ok = active.Value().(bool); !ok {
			return nil, fmt.Errorf("'Active' is not bool: %v", mp)
		}
		isInput, ok := mp["IsInput"]
		if !ok {
			return nil, fmt.Errorf("'IsInput' not found: %v", mp)
		}
		if nodes[i].IsInput, ok = isInput.Value().(bool); !ok {
			return nil, fmt.Errorf("'IsInput' is not bool: %v", mp)
		}
		deviceName, ok := mp["DeviceName"]
		if !ok {
			return nil, fmt.Errorf("'DeviceName' not found: %v", mp)
		}
		if nodes[i].DeviceName, ok = deviceName.Value().(string); !ok {
			return nil, fmt.Errorf("'DeviceName' is not string: %v", mp)
		}
	}
	return nodes, nil
}

// call is a wrapper around CallWithContext for convenience.
func (c *Cras) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
