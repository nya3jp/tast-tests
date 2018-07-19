// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"chromiumos/tast/local/testexec"
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

// ParseUIUseFlags parses conf to get USE flags: empty lines and lines starting
// with # are ignored. Flags can be described by a comment by using # after the
// flag on the same line. If conf is empty, the contents of
// /etc/ui_use_flags.txt are used instead. This is roughly a port of
// get_ui_use_flags() defined in autotest/files/client/bin/utils.py.
func ParseUIUseFlags(conf string) (map[string]bool, error) {
	if conf == "" {
		etcFileContents, err := ioutil.ReadFile("/etc/ui_use_flags.txt")
		if err != nil {
			return nil, err
		}
		conf = string(etcFileContents)
	}
	lines := strings.Split(conf, "\n")
	flags := make(map[string]bool)
	for _, line := range lines {
		if len(line) < 1 || line[0] == '#' {
			continue
		}
		flag := strings.TrimSpace(strings.Split(line, "#")[0])
		if len(flag) > 0 {
			flags[flag] = true
		}
	}
	return flags, nil
}

// Platform returns a string identifying the graphics platform, e.g. 'glx' or
// 'x11_egl' or 'gbm'. This is a port of graphics_platform() defined in
// autotest/files/client/bin/utils.py.
func Platform() string {
	return "null"
}

// Api returns a string identifying the graphics api, e.g. gl or gles2. This is
// roughly a port of graphics_api() defined in
// autotest/files/client/bin/utils.py.
func Api(uiUseFlags map[string]bool) string {
	if uiUseFlags["opengles"] {
		return "gles2"
	}
	return "gl"
}

// WflInfoOptions returns the appropriate options for running the wflinfo
// command. This is roughly a port of wflinfo_cmd() defined in
// autotest/files/client/bin/utils.py.
func WflInfoOptions(platform string, api string) []string {
	return []string{"-p", platform, "-a", api}
}

// GLESVersion returns the OpenGL major and minor versions extracted from the
// output of the wflinfo command. This is roughly a port of get_gles_version()
// defined in autotest/files/client/cros/graphics/graphics_utils.py.
func GLESVersion(ctx context.Context) (int, int, error) {
	// First, run the wflinfo command.
	uiUseFlags, err := ParseUIUseFlags("")
	if err != nil {
		return 0, 0, fmt.Errorf("could not get UI USE flags: %v", err)
	}
	cmd := testexec.CommandContext(ctx, "wflinfo",
		WflInfoOptions(Platform(), Api(uiUseFlags))...)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("running the wflinfo command failed: %v", err)
	}

	// Then, extract the version out of the output. An example of a version
	// string is:
	// OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)
	re, err := regexp.Compile(`OpenGL version string: OpenGL ES ([0-9]+).([0-9]+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if matches == nil || len(matches) != 1 {
		return 0, 0, fmt.Errorf("no OpenGL version string in wflinfo output")
	}
	majorVersion, err := strconv.Atoi(matches[0][1])
	if err != nil {
		return 0, 0, fmt.Errorf("could not parse major version: %v", err)
	}
	minorVersion, err := strconv.Atoi(matches[0][2])
	if err != nil {
		return 0, 0, fmt.Errorf("could not parse minor version: %v", err)
	}
	return majorVersion, minorVersion, nil
}
