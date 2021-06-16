// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"regexp"
	"strings"
)

// Matches "user" or "user@domain".
var emailRegexp = regexp.MustCompile("^([^@]+)(?:@([^@]+))?$")

// NormalizeEmail normalizes the supplied email address as would be done for GAIA login.
// The address is lowercased, periods in the username are removed, and a gmail.com domain is appended if needed.
func NormalizeEmail(email string, removeDots bool) (string, error) {
	matches := emailRegexp.FindStringSubmatch(email)
	if matches == nil {
		return strings.ToLower(email), nil
	}

	user := strings.ToLower(matches[1])
	domain := strings.ToLower(matches[2])
	if domain == "" || domain == "gmail.com" {
		if removeDots {
			user = strings.Replace(user, ".", "", -1)
		}
		domain = "gmail.com"
	}

	return user + "@" + domain, nil
}
