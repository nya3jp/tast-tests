// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
)

const uiUseFlagsPath = "/etc/ui_use_flags.txt"

// The return type of the platform() function.
type PlatformType int

const (
	GLX PlatformType = iota
	X11EGL
	GBM
	UnknownPlatform
)

func (p PlatformType) String() string {
	switch p {
	case GLX:
		return "glx"
	case X11EGL:
		return "x11_egl"
	case GBM:
		return "gbm"
	default:
		return "null"
	}
}

// The return type of the api() function.
type APIType int

const (
	GLES2 APIType = iota
	GL
)

func (p APIType) String() string {
	switch p {
	case GLES2:
		return "gles2"
	default:
		return "gl"
	}
}

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
		if len(l) < 1 || l[0] == '#' {
			continue
		}
		f := strings.TrimSpace(l)
		if len(f) > 0 {
			flags[f] = struct{}{}
		}
	}
	return flags, nil
}

// platform returns a string identifying the graphics platform, e.g. 'glx' or
// 'x11_egl' or 'gbm'. This is a port of graphics_platform() defined in
// autotest/files/client/bin/utils.py.
func platform() PlatformType {
	return UnknownPlatform
}

// api returns a string identifying the graphics api, e.g. gl or gles2. This is
// roughly a port of graphics_api() defined in
// autotest/files/client/bin/utils.py.
func api(uiUseFlags map[string]struct{}) APIType {
	if _, ok := uiUseFlags["opengles"]; ok {
		return GLES2
	}
	return GL
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
	cmd := testexec.CommandContext(ctx, "wflinfo", "-p",
		fmt.Sprintf("%s", platform()), "-a", fmt.Sprintf("%s", api(f)))
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("running the wflinfo command failed: %v", err)
	}

	// Then, extract the version out of the output. An example of a version
	// string is:
	// OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)
	re := regexp.MustCompile(`OpenGL version string: OpenGL ES ([0-9]+).([0-9]+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if matches == nil || len(matches) != 1 {
		return 0, 0, fmt.Errorf("no OpenGL version string (or more than one) found in wflinfo output")
	}
	if major, err = strconv.Atoi(matches[0][1]); err != nil {
		return 0, 0, fmt.Errorf("could not parse major version: %v", err)
	}
	if minor, err = strconv.Atoi(matches[0][2]); err != nil {
		return 0, 0, fmt.Errorf("could not parse minor version: %v", err)
	}
	err = nil
	return
}
