// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
)

const (
	latestFpUpdaterLog   = "/var/log/biod/bio_fw_updater.LATEST"
	previousFpUpdaterLog = "/var/log/biod/bio_fw_updater.PREVIOUS"
)

// ReadFpUpdaterLogs reads the latest and previous fingerprint firmware updater logs.
func ReadFpUpdaterLogs() (string, string, error) {
	latestData, err := ioutil.ReadFile(latestFpUpdaterLog)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to read latest updater log")
	}
	previousData, err := ioutil.ReadFile(previousFpUpdaterLog)
	if err != nil {
		if os.IsNotExist(err) {
			// Previous log doesn't exist, this is the first boot.
			return strings.TrimSpace(string(latestData)), "", nil
		}
		return "", "", errors.Wrap(err, "failed to read previous updater log")
	}
	return strings.TrimSpace(string(latestData)), strings.TrimSpace(string(previousData)), nil
}
