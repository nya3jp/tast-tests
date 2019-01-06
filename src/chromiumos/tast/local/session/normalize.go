// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"strings"

	"chromiumos/tast/errors"
)

// NormalizeEmail normalizes the supplied email address as would be done for GAIA login.
// The address is lowercased, periods in the username are removed, and a gmail.com domain is appended if needed.
func NormalizeEmail(email string) (string, error) {
	var user, domain string
	parts := strings.Split(email, "@")
	if len(parts) == 1 || len(parts) == 2 && strings.ToLower(parts[1]) == "gmail.com" {
		user = strings.Replace(strings.ToLower(parts[0]), ".", "", -1)
		domain = "gmail.com"
	} else if len(parts) == 2 {
		user = strings.ToLower(parts[0])
		domain = strings.ToLower(parts[1])
	} else {
		return "", errors.Errorf("got %d parts", len(parts))
	}

	if user == "" {
		return "", errors.New("missing user")
	}
	return user + "@" + domain, nil
}
