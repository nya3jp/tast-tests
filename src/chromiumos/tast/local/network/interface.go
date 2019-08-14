// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/iw"
)

// GetInterfaceList returns the list of network interfaces.
func GetInterfaceList() ([]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	ifaces := make([]string, len(files))
	for i, file := range files {
		ifaces[i] = file.Name()
	}
	return ifaces, nil
}

// FindWirelessInterface filters interfaces from GetInterfaceList
// by matching against known prefixes. The filtering method will change,
// see crbug.com/988894.
func FindWirelessInterface(ctx context.Context) (string, error) {
	ifs, err := iw.ListInterfaces(ctx)
	if err != nil {
		return "", errors.Wrap(err, "could not get interface list")
	}
	for _, iface := range ifs {
		if mode, err := iw.GetOperatingMode(ctx, iface.IfName); err != nil {
			return "", errors.Wrapf(err, "failed to parse interface mode for %s", iface.IfName)
		} else if mode == "managed" {
			return iface.IfName, nil
		}
	}
	return "", errors.New("could not find a wireless interface")
}
