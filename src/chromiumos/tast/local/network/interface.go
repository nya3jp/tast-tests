// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
)

// Interfaces returns a list of network interfaces.
func Interfaces() ([]string, error) {
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

// WifiInterface returns the first seen WiFi interface.
// It returns "" with error if no Wifi interface is found.
// Currently, it filters interfaces from Interfaces() by prefix match.
// The implementation will be updated (crbug.com/988894).
func WifiInterface() (string, error) {
	ifaces, err := GetInterfaceList()
	if err != nil {
		return "", err
	}
	for _, pref := range []string{"wlan", "mlan"} {
		for _, iface := range ifaces {
			if strings.HasPrefix(iface, pref) {
				return iface, nil
			}
		}
	}
	return "", errors.New("could not find a wireless interface")
}
