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

// Interfaces found in the DUT's /sys/class/net.
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

// WiFiInterfaces found in Interfaces() with "phy80211" property.
func WiFiInterfaces() ([]string, error) {
	ifaces, err := Interfaces()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, iface := range ifaces {
		phy := path.Join(sysfsNet, iface, "phy80211")
		if _, err := os.Stat(phy); err == nil {
			result = append(result, iface)
		}
	}
	return result, nil
}
