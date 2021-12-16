// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package version

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a browser version in the format of "(major).(minor).(build).(patch)".
type Version struct {
	components [4]int64
}

var versionRegexp = regexp.MustCompile(`(\d+).(\d+).(\d+).(\d+)`)

// New creates a new instance of Version with version components.
func New(major, minor, build, patch int64) *Version {
	return &Version{
		components: [4]int64{major, minor, build, patch},
	}
}

// Parse creates a new instance of Version from a given string expression of version.
func Parse(version string) Version {
	v := Version{}
	parts := versionRegexp.FindStringSubmatch(version)
	if parts != nil {
		for id, part := range parts[1:] {
			number, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return Version{}
			}
			v.components[id] = number
		}
	}
	return v
}

// GetString returns a version string. e,g, "99.0.1.0"
func (v *Version) GetString() string {
	var version []string
	for _, component := range v.components {
		version = append(version, strconv.FormatInt(component, 10))
	}
	return strings.Join(version, ".")
}

// Major returns a major version component.
func (v *Version) Major() int64 {
	return v.components[0]
}

// Minor returns a minor version component.
func (v *Version) Minor() int64 {
	return v.components[1]
}

// Build returns a build version component.
func (v *Version) Build() int64 {
	return v.components[2]
}

// Patch returns a patch version component.
func (v *Version) Patch() int64 {
	return v.components[3]
}

// Increment increases version by given components, returns a copy of it.
func (v *Version) Increment(o *Version) Version {
	v.components[0] += o.components[0]
	v.components[1] += o.components[1]
	v.components[2] += o.components[2]
	v.components[3] += o.components[3]
	return *v
}

// Decrement decreases version by given components, returns a copy of it.
func (v *Version) Decrement(o *Version) Version {
	v.components[0] -= o.components[0]
	v.components[1] -= o.components[1]
	v.components[2] -= o.components[2]
	v.components[3] -= o.components[3]
	return *v
}

// IsNewerThan compares two version and returns true when lhs is newer than rhs.
func (v *Version) IsNewerThan(rhs Version) bool {
	for i, component := range v.components {
		o := rhs.components[i]
		if component != o {
			return component > o
		}
	}
	return false
}

// IsOlderThan compares two version and returns true when lhs is older than rhs.
func (v *Version) IsOlderThan(rhs Version) bool {
	return rhs.IsNewerThan(*v)
}

// IsEqualTo returns true when the two versions are the same.
func (v *Version) IsEqualTo(rhs Version) bool {
	return v.components[0] == rhs.components[0] && v.components[1] == rhs.components[1] && v.components[2] == rhs.components[2] && v.components[3] == rhs.components[3]
}

// IsValid checks if the version is set with valid numbers.
func (v *Version) IsValid() bool {
	return v.GetString() != "0.0.0.0"
}

// IsSkewValid returns whether it is a valid version skew that is compatible with the given ash/OS version.
func (v *Version) IsSkewValid(ash Version) bool {
	// TODO(crbug.com/1258138): Update version skew policy for Tast. Currently, it is [-1, inf] but should be [0, +2] as soon as the issue is resolved.
	// Note that this version skew policy should be in line with the production code.
	// See LacrosInstallerPolicy::ComponentReady at
	//   https://osscs.corp.google.com/chromium/chromium/src/+/main:chrome/browser/component_updater/cros_component_installer_chromeos.cc
	const minMajorVersionSkew = -1
	return v.components[0] >= ash.components[0]+minMajorVersionSkew &&
		v.components[0] <= math.MaxInt64
}
