// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package params

import (
	"encoding/json"
	"io/ioutil"
	"strconv"

	"chromiumos/tast/errors"
)

// Configuration contains the expected state of Omaha.
type Configuration struct {
	// ChromeOSVersionFromMilestone maps Chrome milestones to ChromeOS version prefixes.
	ChromeOSVersionFromMilestone map[int]int

	// ChromeOSLTRMilestoneWithMinimumMinor maps LTS milestones to LTR-only minor versions.
	ChromeOSLTRMilestoneWithMinimumMinor map[int]int
}

// DumpToFile writes the device parameters to a file.
func (d *Configuration) DumpToFile(path string) error {
	file, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal configuration")
	}

	return ioutil.WriteFile(path, file, 0644)
}

// PreviousMilestone returns the previous milestone if the config contains it.
func (d *Configuration) PreviousMilestone(milestone int) (int, error) {
	if _, ok := d.ChromeOSVersionFromMilestone[milestone-1]; ok {
		return milestone - 1, nil
	}

	return 0, errors.Errorf("no previous milestones of %d found", milestone)
}

// NextMilestone returns the next milestone if the config contains it.
func (d *Configuration) NextMilestone(milestone int) (int, error) {
	if _, ok := d.ChromeOSVersionFromMilestone[milestone+1]; ok {
		return milestone + 1, nil
	}
	return 0, errors.Errorf("no next milestones of %d found", milestone)
}

// PreviousMilestoneOSVersion generates a ChromeOS version that matches the
// previous milestone prefix.
func (d *Configuration) PreviousMilestoneOSVersion(milestone int) (string, error) {
	m, err := d.PreviousMilestone(milestone)
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(int64(d.ChromeOSVersionFromMilestone[m]), 10) + ".0.0", nil
}

// NextMilestoneOSVersion generates a ChromeOS version that matches the
// next milestone prefix.
func (d *Configuration) NextMilestoneOSVersion(milestone int) (string, error) {
	m, err := d.NextMilestone(milestone)
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(int64(d.ChromeOSVersionFromMilestone[m]), 10) + ".0.0", nil
}

// CurrentChromeOSStable returns the current ChromeOS stable milestone.
// If not entries are found, returns -1.
func (d *Configuration) CurrentChromeOSStable() int {
	currentChromeOSStable := -1
	for n := range d.ChromeOSVersionFromMilestone {
		if n > currentChromeOSStable {
			currentChromeOSStable = n
		}
	}

	return currentChromeOSStable
}
