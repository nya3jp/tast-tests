// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"io/ioutil"

	"chromiumos/tast/errors"
)

// Interfaces returns the list of network interfaces sorted by name.
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
