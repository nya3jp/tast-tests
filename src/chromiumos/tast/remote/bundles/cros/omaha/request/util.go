// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"strconv"
	"strings"
)

// MatchOneOfVersions checks if a given version matches any of the provided major versions.
func MatchOneOfVersions(version string, majors ...int) bool {
	for _, major := range majors {
		majorStr := strconv.FormatInt(int64(major), 10)
		if strings.HasPrefix(version, majorStr+".") {
			return true
		}
	}

	return false
}
