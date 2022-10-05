// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package params

import (
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
)

// Configuration contains the expected state of Omaha.
type Configuration struct {
	// OldVersion is a version older that then any available stable and should
	// update to the latest stable without any stepping stone.
	OldVersion string

	// ChromeOSVersionFromMilestone maps Chrome milestones to ChromeOS version prefixes.
	ChromeOSVersionFromMilestone map[int]int

	// ChromeOSLTRMilestoneWithMininumMinor lists the LTS milestones and maps them to the a minor version that is LTS only.
	ChromeOSLTRMilestoneWithMininumMinor map[int]int

	// CurrentStableChrome is the current stable milestone of ChromeOS.
	CurrentStableChrome int
	// CurrentStableChrome is the next stable milestone of ChromeOS.
	NextStableChrome int
}

// DumpToFile writes the device parameters to a file.
func (d *Configuration) DumpToFile(path string) error {
	file, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal configuration")
	}

	return ioutil.WriteFile(path, file, 0644)
}
