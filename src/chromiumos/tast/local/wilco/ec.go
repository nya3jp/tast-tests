// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
)

const (
	eventTriggerPath = "/sys/kernel/debug/wilco_ec/test_event"
	eventReadPath    = "/dev/wilco_event0"
)

// TriggerECEvent writes data to the EC event trigger path and triggers a dummy
// EC event.
func TriggerECEvent() error {
	if err := ioutil.WriteFile(eventTriggerPath, []byte{0}, 0644); err != nil {
		return errors.Wrapf(err, "failed to write to %v", eventTriggerPath)
	}
	return nil
}

// ReadECData reads up to the size of the size of provided byte slice from the
// EC event device node. The number of bytes read will be returned.
func ReadECData(data []byte) (int, error) {
	f, err := os.OpenFile(eventReadPath, os.O_RDONLY, 0644)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to open %v", eventReadPath)
	}
	defer f.Close()

	n, err := f.Read(data)
	if err != nil {
		return n, errors.Wrapf(err, "failed to read from %v", eventReadPath)
	}

	return n, nil
}
