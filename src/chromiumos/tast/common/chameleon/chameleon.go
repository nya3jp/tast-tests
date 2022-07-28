// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import (
	"context"

	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Chameleon holds the chameleond connection information.
type Chameleon struct {
	xmlrpc *xmlrpc.XMLRpc
	ports  []int // supported ports
}

// New creates a new Chameleon object for communicating with a chameleond instance.
// connSpec holds chameleond's location, either as "host:port" or just "host"
// (to use the default port).
// Deprecated: Use NewChameleond instead, which uses the more complete
// chameleond API, Chameleond.
func New(ctx context.Context, connSpec string) (*Chameleon, error) {
	testing.ContextLogf(ctx, "New chameloen - conSpec: %s", connSpec)
	host, port, err := parseConnSpec(connSpec)
	if err != nil {
		return nil, err
	}
	ch := &Chameleon{xmlrpc: xmlrpc.New(host, port)}
	ports, err := ch.SupportedPorts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to communicate with chameleon board to get supported ports")
	}
	ch.ports = ports
	return ch, nil
}

// SupportedPorts calls the Chameleon GetSupportedPorts method.
func (ch *Chameleon) SupportedPorts(ctx context.Context) ([]int, error) {
	var ports []int
	if err := ch.xmlrpc.Run(ctx, xmlrpc.NewCall("GetSupportedPorts"), &ports); err != nil {
		return nil, err
	}
	return ports, nil
}

// Close releases the chameleon board resources.
func (ch *Chameleon) Close(ctx context.Context) error {
	return nil
}
