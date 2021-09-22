// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"strconv"
	"strings"
)

// Version represents a browser version in the format of "(major).(minor).(build).(patch)".
type Version struct {
	components [4]uint64
}

// New creates a new instance of Version with version components.
func New(major, minor, build, patch uint64) Version {
	return Version{
		components: [4]uint64{major, minor, build, patch},
	}
}

// Parse creates a new instance of Version from a given string expression of version.
func Parse(version string) Version {
	v := Version{}
	parts := strings.Split(version, ".")
	if len(parts) == 4 {
		for id, part := range parts {
			number, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return Version{}
			}
			v.components[id] = number
		}
	}
	return v
}

// GetString returns a version string. e,g, "99.0.1.0"
func (v Version) GetString() string {
	var version []string
	for _, component := range v.components {
		version = append(version, strconv.FormatUint(component, 10))
	}
	return strings.Join(version, ".")
}

// Increment increases version by given components, returns a copy of it.
func (v *Version) Increment(major, minor, build, patch uint64) Version {
	v.components[0] += major
	v.components[1] += minor
	v.components[2] += build
	v.components[3] += patch
	return *v
}

// Decrement decreases version by given components, returns a copy of it.
func (v *Version) Decrement(major, minor, build, patch uint64) Version {
	v.components[0] -= major
	v.components[1] -= minor
	v.components[2] -= build
	v.components[3] -= patch
	return *v
}

// IsValid checks if the version is set with valid numbers.
func (v Version) IsValid() bool {
	return v.GetString() != "0.0.0.0"
}
