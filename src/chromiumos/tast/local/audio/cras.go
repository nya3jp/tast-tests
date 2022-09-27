// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio interacts with audio operation.
package audio

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

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

// VolumeState contains the metadata of volume state in Cras.
// Currently fields which are actually needed by tests are defined.
// Please find src/third_party/adhd/cras/dbus_bindings/org.chromium.cras.Control.xml
// for the meaning of each fields.
type VolumeState struct {
	OutputVol      int
	OutputMute     bool
	InputMute      bool
	OutputUserMute bool
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

// GetNodeByType returns the first node with given type.
func (c *Cras) GetNodeByType(ctx context.Context, t string) (*CrasNode, error) {
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		if n.Type == t {
			return &n, nil
		}
		// Regard the front mic as the internal mic.
		if t == "INTERNAL_MIC" && n.Type == "FRONT_MIC" {
			return &n, nil
		}
	}

	return nil, errors.Errorf("failed to find a node with type %s", t)
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
	return c.call(ctx, cmd, node.ID).Err
}

// SetActiveNodeByType sets node with specified type active.
func (c *Cras) SetActiveNodeByType(ctx context.Context, nodeType string) error {
	var node *CrasNode

	// Wait until the node with this type is existing.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := c.GetNodeByType(ctx, nodeType)
		node = n
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Errorf("failed to wait node %s", nodeType)
	}

	if err := c.SetActiveNode(ctx, *node); err != nil {
		return errors.Errorf("failed to set node %s active", nodeType)
	}

	// Wait until that node is active.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		n, err := c.GetNodeByType(ctx, nodeType)
		if err != nil {
			return err
		}
		if !n.Active {
			return errors.New("node is not active")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Errorf("failed to wait node %s to be active", nodeType)
	}

	return nil
}

// SetOutputNodeVolume calls cras.Control.SetOutputNodeVolume over D-Bus.
func (c *Cras) SetOutputNodeVolume(ctx context.Context, node CrasNode, volume int) error {
	return c.call(ctx, "SetOutputNodeVolume", node.ID, volume).Err
}

// GetVolumeState calls cras.Control.GetVolumeState over D-Bus.
func (c *Cras) GetVolumeState(ctx context.Context) (*VolumeState, error) {
	var vol int32
	var outputMute, inputMute, outputUserMute bool

	err := c.call(ctx, "GetVolumeState").Store(&vol, &outputMute, &inputMute, &outputUserMute)
	if err != nil {
		return nil, err
	}

	return &VolumeState{
		OutputVol:      int(vol),
		OutputMute:     outputMute,
		InputMute:      inputMute,
		OutputUserMute: outputUserMute,
	}, nil
}

// WaitForDeviceUntil waits until any cras node meets the given condition.
// condition is a function that takes a cras node as input and returns true if the node status
// satisfies the criteria.
func (c *Cras) WaitForDeviceUntil(ctx context.Context, condition func(*CrasNode) bool, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		nodes, err := c.GetNodes(ctx)
		if err != nil {
			return err
		}

		for _, n := range nodes {
			if condition(&n) {
				return nil
			}
		}
		return errors.New("cras node(s) not in requested condition")
	}, &testing.PollOptions{Timeout: timeout, Interval: 1 * time.Second})
}

// WaitForDevice waits for specified types of stream nodes to be active.
// You can pass the streamType as a bitmap to wait for both input and output
// nodes to be active. Ex: WaitForDevice(ctx, InputStream|OutputStream)
// It should be used to verify the target types of nodes exist and are
// active before the real test starts.
// Notice that some devices use their displays as an internal speaker
// (e.g. monroe). When a display is closed, the internal speaker is removed,
// too. For this case, we should call power.TurnOnDisplay to turn on a display
// to re-enable an internal speaker.
func WaitForDevice(ctx context.Context, streamType StreamType) error {
	cras, err := NewCras(ctx)
	if err != nil {
		return err
	}

	var active StreamType
	checkActiveNode := func(n *CrasNode) bool {
		if !n.Active {
			return false
		}
		if n.IsInput {
			active |= InputStream
		} else {
			active |= OutputStream
		}
		return streamType&active == streamType
	}

	return cras.WaitForDeviceUntil(ctx, checkActiveNode, 10*time.Second)
}

// SelectedOutputDevice returns the active output device name and type.
func (c *Cras) SelectedOutputDevice(ctx context.Context) (deviceName, deviceType string, err error) {
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return
	}
	for _, node := range nodes {
		if node.Active && !node.IsInput {
			deviceName = node.DeviceName
			deviceType = node.Type
			break
		}
	}
	return
}
