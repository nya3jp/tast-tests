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

// NewNetworkHealth creates a connection to network_health that allows to make
// mojo calls. It receives a connection where it is possible to create a mojo
// connection to network_health.
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
	var result struct{ Result []Network }

	s.Log("In GetNetworkList")
	if err := n.mojoRemote.Call(ctx, &result,
	"function test() { console.log('Printing message inside GetNetworkList'); var res = this.getNetworkList(); console.log(res); return res;}"); err != nil {
		return result.Result, errors.Wrap(err, "failed to run GetNetworkList")
	}
	s.Log("Done with with mojo remote call")

	return result.Result, nil
}
