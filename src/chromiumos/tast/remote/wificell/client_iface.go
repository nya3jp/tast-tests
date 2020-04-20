// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
)

// ClientIface provides methods to access properties of test client (DUT)'s network interface.
type ClientIface struct {
	Name string // Network interface name.
}

// NewClientInterface creates a ClientIface.
func (tf *TestFixture) NewClientInterface(ctx context.Context) (*ClientIface, error) {
	netIf, err := tf.wifiClient.GetInterface(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface name")
	}
	return &ClientIface{Name: netIf.Name}, nil
}
