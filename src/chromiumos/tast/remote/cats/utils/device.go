// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"cienet.com/cats/node/sdk"
)

// Device designates a DUT.
type Device struct {
	// Client is C-ATS node client.
	Client sdk.DelegateClient
	// DeviceID is Android Device ID.
	DeviceID string
}

// NewDevice returns a Device struct.
func NewDevice(client sdk.DelegateClient, deviceID string) *Device {
	return &Device{
		Client:   client,
		DeviceID: deviceID,
	}
}
