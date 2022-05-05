// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Capabilities is the SupportedCapabilities property in LEAdvertisingManager1
// interface, which contains Advertising-related controller capabilities.
type Capabilities struct {
	MaxAdvLen    uint8
	MaxScnRspLen uint8
	MaxTxPower   int16
	MinTxPower   int16
}

const advertisingsIface = service + ".LEAdvertisingManager1"

// SupportedCapabilities returns the supportedCapabilities of the adapter.
func (a *Adapter) SupportedCapabilities(ctx context.Context) (Capabilities, error) {
	const prop = advertisingsIface + ".SupportedCapabilities"
	value, err := dbusutil.Property(ctx, a.obj, prop)
	if err != nil {
		return Capabilities{}, err
	}
	supportedCapabilities, ok := value.(map[string]dbus.Variant)
	if !ok {
		return Capabilities{}, errors.New("supportedCapabilities property not a string to dbus.Variant map")
	}
	maxAdvLen, ok := supportedCapabilities["MaxAdvLen"].Value().(uint8)
	if !ok {
		return Capabilities{}, errors.New("MaxAdvLen in supportedCapabilities property not a uint8")
	}
	maxScnRspLen, ok := supportedCapabilities["MaxScnRspLen"].Value().(uint8)
	if !ok {
		return Capabilities{}, errors.New("MaxScnRspLen in supportedCapabilities property not a uint8")
	}
	maxTxPower, ok := supportedCapabilities["MaxTxPower"].Value().(int16)
	if !ok {
		return Capabilities{}, errors.New("MaxTxPower in supportedCapabilities property not a int16")
	}
	minTxPower, ok := supportedCapabilities["MinTxPower"].Value().(int16)
	if !ok {
		return Capabilities{}, errors.New("MinTxPower in supportedCapabilities property not a int16")
	}

	return Capabilities{
		MaxAdvLen:    maxAdvLen,
		MaxScnRspLen: maxScnRspLen,
		MaxTxPower:   maxTxPower,
		MinTxPower:   minTxPower,
	}, nil
}
