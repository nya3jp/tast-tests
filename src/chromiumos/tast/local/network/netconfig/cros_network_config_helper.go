// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netconfig contains the mojo connection to cros_network_config.
package netconfig

// This file containes helper functions for commonly used patterns in mojo

// NetworkStateIsConnectedOrOnline checks whether network is connected or online
func NetworkStateIsConnectedOrOnline(networkState NetworkStateProperties) bool {
	return networkState.ConnectionState == ConnectedCST || networkState.ConnectionState == OnlineCST
}
