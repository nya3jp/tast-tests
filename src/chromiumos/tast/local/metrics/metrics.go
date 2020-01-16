// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

const (
	legacyConsent = "/home/chronos/Consent To Send Stats"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// HasConsent checks if the system has metrics consent.
func HasConsent() (bool, error) {
	// Logic here should be in sync with ConsentId in libmetrics.
	b, err := ioutil.ReadFile(legacyConsent)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "failed to examine legacy consent file: %s", legacyConsent)
	}
	s := strings.TrimRight(string(b), "\n")
	return uuidPattern.MatchString(s), nil
}
