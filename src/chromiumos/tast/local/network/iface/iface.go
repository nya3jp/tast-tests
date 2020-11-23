// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iface contains utility functions for a network interface.
package iface

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

// Iface is the object contains the interface name.
type Iface struct {
	name string
}

// NewInterface creates a new Iface object.
func NewInterface(n string) *Iface {
	return &Iface{name: n}
}

const deviceInfoRoot = "/sys/class/net"

// ParentDeviceName returns name of device at which wiphy device is present.
func (i *Iface) ParentDeviceName(ctx context.Context) (string, error) {
	devicePath := filepath.Join(deviceInfoRoot, i.name, "device")
	rel, err := os.Readlink(devicePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to readlink device path")
	}

	deviceName := filepath.Base(rel)
	return deviceName, nil
}

// PhyName returns name of the WiFi phy (e.g., "phy0").
func (i *Iface) PhyName(_ context.Context) (string, error) {
	p, err := os.Readlink(filepath.Join(deviceInfoRoot, i.name, "phy80211"))
	if err != nil {
		return "", errors.Wrap(err, "failed to readlink phy path")
	}
	return filepath.Base(p), nil
}
