// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"io/ioutil"
	"os"
	"path"

	"chromiumos/tast/errors"
)

const sysfsNet = "/sys/class/net"

// Interfaces returns a list of network interfaces.
func Interfaces() ([]string, error) {
	files, err := ioutil.ReadDir(sysfsNet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	ifaces := make([]string, len(files))
	for i, file := range files {
		ifaces[i] = file.Name()
	}
	return ifaces, nil
}

// WifiInterfaces returns a list of WiFi interfaces.
func WifiInterfaces() ([]string, error) {
	ifaces, err := Interfaces()
	if err != nil {
		return nil, err
	}
	numWifi := 0
	for _, iface := range ifaces {
		phy := path.Join(sysfsNet, iface, "phy80211")
		if _, err := os.Stat(phy); err == nil {
			ifaces[numWifi] = iface
			numWifi++
		}
	}
	ifaces = ifaces[:numWifi]
	return ifaces, nil
}
