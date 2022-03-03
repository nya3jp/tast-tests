// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netconfig

import "chromiumos/tast/errors"

// This file contains helper functions for commonly used patterns that occur in connectivity tests using mojo.

// NetworkStateIsConnectedOrOnline checks whether network is connected or online.
func NetworkStateIsConnectedOrOnline(networkState NetworkStateProperties) (bool, error) {
	if networkState != nil {
		return networkState.ConnectionState == ConnectedCST || networkState.ConnectionState == OnlineCST, nil
	}
	return errors.New("networkState is Nil")
}
