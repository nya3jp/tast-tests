// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package health contains the mojo connection to network_health.
package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// NetworkHealth contains the mojo connection to network_health.
type NetworkHealth struct {
	conn       *chrome.Conn
	mojoRemote *chrome.JSObject
}

// CreateLoggedInNetworkHealth creates a connection to network_health when the
// device is logged in so chrome://network may be opened.
func CreateLoggedInNetworkHealth(ctx context.Context, cr *chrome.Chrome) (*NetworkHealth, error) {
	conn, err := cr.NewConn(ctx, "chrome://network")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open network tab")
	}

	return NewNetworkHealth(ctx, conn)
}

// NewNetworkHealth creates a connection to the network_health mojo API,
// allowing it to make calls to the NetworkHeathService Mojo interface.
func NewNetworkHealth(ctx context.Context, conn *chrome.Conn) (*NetworkHealth, error) {
	var mojoRemote chrome.JSObject
	if err := conn.Call(ctx, &mojoRemote, networkHealthJs); err != nil {
		return nil, errors.Wrap(err, "failed to set up the network health mojo API")
	}

	return &NetworkHealth{conn, &mojoRemote}, nil
}

// Close cleans up the injected javascript.
func (n *NetworkHealth) Close(ctx context.Context) error {
	if err := n.mojoRemote.Release(ctx); err != nil {
		return err
	}
	return n.conn.Close()
}

// GetNetworkList returns an array of Network structs.
func (n *NetworkHealth) GetNetworkList(ctx context.Context, s *testing.State) ([]Network, error) {
	var result []Network
	if err := n.mojoRemote.Call(ctx, &result,
		"async function() { var res = await this.getNetworkList(); return res.networks}"); err != nil {
		return result, errors.Wrap(err, "failed to run GetNetworkList")
	}

	return result, nil
}
