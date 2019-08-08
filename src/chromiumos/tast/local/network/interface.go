// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
)

// GetInterfaceList returns the list of network interfaces.
func GetInterfaceList() ([]string, error) {
	files, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get interfaces")
	}
	toRet := make([]string, len(files))
	for i, file := range files {
		toRet[i] = file.Name()
	}
	return toRet, nil
}

// FindWirelessInterface filters interfaces from GetInterfaceList
// by matching against known prefixes. The filtering method will change,
// see crbug.com/988894.
func FindWirelessInterface() (string, error) {
	typeList := []string{"wlan", "mlan"}
	ifaceList, err := GetInterfaceList()
	if err != nil {
		return "", err
	}
	for _, pref := range typeList {
		for _, iface := range ifaceList {
			if strings.HasPrefix(iface, pref) {
				return iface, nil
			}
		}
	}
	return "", errors.New("could not find a wireless interface")
}
