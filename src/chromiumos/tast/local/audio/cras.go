// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.cras"
	dbusPath      = "/org/chromium/cras"
	dbusInterface = "org.chromium.cras.Control"
)

// StreamType is used to specify the type of node we want to use for tests and
// helper functions.
type StreamType uint

const (
	// InputStream describes nodes with true IsInput attributes.
	InputStream StreamType = 1 << iota
	// OutputStream describes nodes with false IsInput attributes.
	OutputStream
)

func (t StreamType) String() string {
	switch t {
	case InputStream:
		return "InputStream"
	case OutputStream:
		return "OutputStream"
	default:
		return fmt.Sprintf("StreamType(%#x)", t)
	}
}

// Cras is used to interact with the cras process over D-Bus.
// For detailed spec, please find src/third_party/adhd/cras/README.dbus-api.
type Cras struct {
	obj dbus.BusObject
}

// NewCras connects to CRAS via D-Bus and returns a Cras object.
func NewCras(ctx context.Context) (*Cras, error) {
	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusName)
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Cras{obj}, nil
}

// CrasNode contains the metadata of Node in Cras.
// Currently fields which are actually needed by tests are defined.
// Please find src/third_party/adhd/cras/README.dbus-api for the meaning of
// each fields.
type CrasNode struct {
	ID         uint64
	Type       string
	Active     bool
	IsInput    bool
	DeviceName string
	NodeVolume uint64
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
	// That usage is not common practice, and it is less supported in
	// godbus. Here, instead, values are manually converted via
	// dbus.Variant.
	nodes := make([]CrasNode, len(call.Body))
	for i, n := range call.Body {
		mp := n.(map[string]dbus.Variant)
		if id, ok := mp["Id"]; !ok {
			return nil, errors.Errorf("'Id' not found: %v", mp)
		} else if nodes[i].ID, ok = id.Value().(uint64); !ok {
			return nil, errors.Errorf("'Id' is not uint64: %v", mp)
		}
		if nodeType, ok := mp["Type"]; !ok {
			return nil, errors.Errorf("'Type' not found: %v", mp)
		} else if nodes[i].Type, ok = nodeType.Value().(string); !ok {
			return nil, errors.Errorf("'Type' is not string: %v", mp)
		}
		if active, ok := mp["Active"]; !ok {
			return nil, errors.Errorf("'Active' not found: %v", mp)
		} else if nodes[i].Active, ok = active.Value().(bool); !ok {
			return nil, errors.Errorf("'Active' is not bool: %v", mp)
		}
		if isInput, ok := mp["IsInput"]; !ok {
			return nil, errors.Errorf("'IsInput' not found: %v", mp)
		} else if nodes[i].IsInput, ok = isInput.Value().(bool); !ok {
			return nil, errors.Errorf("'IsInput' is not bool: %v", mp)
		}
		if deviceName, ok := mp["DeviceName"]; !ok {
			return nil, errors.Errorf("'DeviceName' not found: %v", mp)
		} else if nodes[i].DeviceName, ok = deviceName.Value().(string); !ok {
			return nil, errors.Errorf("'DeviceName' is not string: %v", mp)
		}
		if nodeVolume, ok := mp["NodeVolume"]; !ok {
			return nil, errors.Errorf("'NodeVolume' not found: %v", mp)
		} else if nodes[i].NodeVolume, ok = nodeVolume.Value().(uint64); !ok {
			return nil, errors.Errorf("'NodeVolume' is not uint64: %v", mp)
		}
	}
	return nodes, nil
}

// call is a wrapper around CallWithContext for convenience.
func (c *Cras) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// SetActiveNode calls cras.Control.SetActiveInput(Output)Node over D-Bus.
func (c *Cras) SetActiveNode(ctx context.Context, node CrasNode) error {
	cmd := "SetActiveOutputNode"
	if node.IsInput {
		cmd = "SetActiveInputNode"
	}
	call := c.call(ctx, cmd, node.ID)
	return call.Err
}

// SetActiveNodeByType sets node with specified type active.
func SetActiveNodeByType(ctx context.Context, nodeType string) error {
	cras, err := NewCras(ctx)
	if err != nil {
		return err
	}
	crasNodes, err := cras.GetNodes(ctx)
	if err != nil {
		return err
	}

	for _, n := range crasNodes {
		if n.Type == nodeType {
			return cras.SetActiveNode(ctx, n)
		}
	}
	return errors.Errorf("node(s) %+v not contain requested type", crasNodes)
}

// WaitForDevice waits for specified types of stream nodes to be active.
// You can pass the streamType as a bitmap to wait for both input and output
// nodes to be active. Ex: WaitForDevice(ctx, InputStream|OutputStream)
// It should be used to verify the target types of nodes exist and are
// active before the real test starts.
func WaitForDevice(ctx context.Context, streamType StreamType) error {
	checkActiveNodes := func(ctx context.Context) error {
		cras, err := NewCras(ctx)
		if err != nil {
			return err
		}
		crasNodes, err := cras.GetNodes(ctx)
		if err != nil {
			return err
		}

		var active StreamType
		for _, n := range crasNodes {
			if n.Active {
				if n.IsInput {
					active |= InputStream
				} else {
					active |= OutputStream
				}

				if streamType&active == streamType {
					return nil
				}
			}
		}

		return errors.Errorf("node(s) %+v not in requested state", crasNodes)
	}

	return testing.Poll(ctx, checkActiveNodes, &testing.PollOptions{Timeout: 5 * time.Second})
}
