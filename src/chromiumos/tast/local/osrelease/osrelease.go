// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package osrelease provides a parser of /etc/os-release and
// /etc/os-release.d/ if any.
package osrelease

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Path is the primary path for os-release file.
const Path = "/etc/os-release"

// OverridePath is directory path for os-release.d/.
const OverridePath = "/etc/os-release.d/"

// Keys in /etc/os-release. See the following doc for details:
// https://chromium.googlesource.com/chromiumos/docs/+/HEAD/os_config.md#os-release
const (
	// Site where to report bugs (e.g. "https://crbug.com/new")
	BugReportURL = "BUG_REPORT_URL"

	// Full OS version (e.g. "11012.0.2018_08_28_1422")
	BuildID = "BUILD_ID"

	// ID used when reporting crashes (e.g. "ChromeOS")
	GoogleCrashID = "GOOGLE_CRASH_ID"

	// Site where people can find info about this project
	// (e.g. "https://www.chromium.org/chromium-os")
	HomeURL = "HOME_URL"

	// Unique ID for the OS (e.g. "chromeos")
	ID = "ID"

	// Unique IDs that the OS is related to (e.g. "chromiumos")
	IDLike = "ID_LIKE"

	// For name for the OS (e.g. "Chrome OS")
	Name = "NAME"

	// Major release version (e.g. "70")
	Version = "VERSION"

	// Full release version (e.g. "70")
	VersionID = "VERSION_ID"
)

// Load loads /etc/os-release and /etc/os-release.d/ if any.
// Return a parsed key-value map.
func Load() (map[string]string, error) {
	file, err := os.Open(Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Overrides are optional.
	overrides := make(map[string]string)
	d, err := ioutil.ReadDir(OverridePath)
	if err == nil {
		for _, file := range d {
			key := file.Name()
			path := filepath.Join(OverridePath, key)

			content, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}

			value := strings.TrimSpace(string(content))
			overrides[key] = value
		}
	}

	return Parse(file, overrides)
}

// lineRe matches a key-value line after surrounding whitespaces in /etc/os-release.
var lineRe = regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*(.*)$`)

// Parse parses a key-value text file in the /etc/os-release format and overrides in
// /etc/os-release.d/ if any.
// Return a parsed key-value map.
func Parse(file io.Reader, overrides map[string]string) (map[string]string, error) {
	res := make(map[string]string)

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		res[m[1]] = m[2]
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	for k, v := range overrides {
		res[k] = v
	}

	return res, nil
}
