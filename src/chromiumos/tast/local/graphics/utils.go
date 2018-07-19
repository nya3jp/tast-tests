// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const uiUseFlagsPath = "/etc/ui_use_flags.txt"

// parseUIUseFlags parses the configuration file located at path to get the UI
// USE flags: empty lines and lines starting with # are ignored. No end-of-line
// comments should be used. This is roughly a port of get_ui_use_flags() defined
// in autotest/files/client/bin/utils.py.
func parseUIUseFlags(path string) (map[string]struct{}, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	flags := make(map[string]struct{})
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) > 0 && l[0] != '#' {
			flags[l] = struct{}{}
		}
	}
	return flags, nil
}

// api returns a string identifying the graphics api, e.g. gl or gles2. This is
// roughly a port of graphics_api() defined in
// autotest/files/client/bin/utils.py.
func api(uiUseFlags map[string]struct{}) string {
	if _, ok := uiUseFlags["opengles"]; ok {
		return "gles2"
	}
	return "gl"
}

// glesVersion returns the OpenGL major and minor versions extracted from the
// output of the wflinfo command. This is roughly a port of get_gles_version()
// defined in autotest/files/client/cros/graphics/graphics_utils.py.
func glesVersion(ctx context.Context) (major int, minor int, err error) {
	// First, run the wflinfo command.
	f, err := parseUIUseFlags(uiUseFlagsPath)
	if err != nil {
		return 0, 0, fmt.Errorf("could not get UI USE flags: %v", err)
	}
	cmd := testexec.CommandContext(ctx, "wflinfo", "-p", "null", "-a", api(f))
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return 0, 0, fmt.Errorf("running the wflinfo command failed: %v", err)
	}

	// Then, extract the version out of the output. An example of a version
	// string is:
	// OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)
	re := regexp.MustCompile(
		`OpenGL version string: OpenGL ES ([0-9]+).([0-9]+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if len(matches) != 1 {
		testing.ContextLog(ctx, "Output of wflinfo:\n", string(out))
		return 0, 0, fmt.Errorf(
			"%d OpenGL version strings found in wflinfo output", len(matches))
	}
	testing.ContextLogf(ctx, "Got %q", matches[0][0])
	if major, err = strconv.Atoi(matches[0][1]); err != nil {
		return 0, 0, fmt.Errorf("could not parse major version: %v", err)
	}
	if minor, err = strconv.Atoi(matches[0][2]); err != nil {
		return 0, 0, fmt.Errorf("could not parse minor version: %v", err)
	}
	return major, minor, nil
}
