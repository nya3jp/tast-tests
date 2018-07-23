// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const deqpBaseDir = "/usr/local/deqp"
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

// extractOpenGLVersion takes the output of the wflinfo command and attempts to
// extract the OpenGL version. An example of the OpenGL version string expected
// in the wflinfo output is:
// OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)
func extractOpenGLVersion(ctx context.Context, wflout string) (major int,
	minor int, err error) {
	re := regexp.MustCompile(
		`OpenGL version string: OpenGL ES ([0-9]+).([0-9]+)`)
	matches := re.FindAllStringSubmatch(wflout, -1)
	if len(matches) != 1 {
		testing.ContextLog(ctx, "Output of wflinfo:\n", wflout)
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

// GLESVersion returns the OpenGL major and minor versions extracted from the
// output of the wflinfo command. This is roughly a port of get_gles_version()
// defined in autotest/files/client/cros/graphics/graphics_utils.py.
func GLESVersion(ctx context.Context) (major int, minor int, err error) {
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
	return extractOpenGLVersion(ctx, string(out))
}

// SupportsVulkanForDEQP decides whether the board supports Vulkan for DEQP
// testing. An error is returned if something unexpected happens while deciding.
// This is a port of part of the functionality of GraphicsApiHelper defined in
// autotest/files/client/cros/graphics/graphics_utils.py.
func SupportsVulkanForDEQP(ctx context.Context) (bool, error) {
	// First, search for libvulkan.so.
	hasVulkan := false
	for _, dir := range []string{"/usr/lib", "/usr/lib64", "/usr/local/lib",
		"/usr/local/lib64"} {
		if f, err := os.Open(filepath.Join(dir, "libvulkan.so")); err == nil {
			f.Close()
			hasVulkan = true
			break
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("libvulkan.so search error: %v", err)
		}
	}
	if !hasVulkan {
		testing.ContextLog(ctx, "Could not find libvulkan.so")
		return false, nil
	}

	// Then, search for the deqp-vk testing binary.
	p, ok := DEQPExecutable("vk")
	if !ok {
		return false, fmt.Errorf("could not get the path for the 'vk' API")
	}
	if f, err := os.Open(p); err == nil {
		f.Close()
		return true, nil
	}

	testing.ContextLog(ctx, "Found libvulkan.so but not the deqp-vk binary")
	return false, nil
}

// SupportedAPIs returns an array of supported API names given the OpenGL
// version and whether Vulkan is supported. This is a port of part of the
// functionality of GraphicsApiHelper defined in
// autotest/files/client/cros/graphics/graphics_utils.py.
func SupportedAPIs(glMajor int, glMinor int, vulkan bool) []string {
	apis := []string{}
	if glMajor >= 2 {
		apis = append(apis, "gles2")
	}
	if glMajor >= 3 {
		apis = append(apis, "gles3")
		if glMajor > 3 || glMinor >= 1 {
			apis = append(apis, "gles31")
		}
	}
	if vulkan {
		apis = append(apis, "vk")
	}
	return apis
}

// DEQPExecutable returns the path to the executable corresponding to an API
// name. This is a port of part of the functionality of GraphicsApiHelper
// defined in autotest/files/client/cros/graphics/graphics_utils.py.
func DEQPExecutable(api string) (string, bool) {
	p, ok := map[string]string{
		"gles2":  filepath.Join(deqpBaseDir, "modules/gles2/deqp-gles2"),
		"gles3":  filepath.Join(deqpBaseDir, "modules/gles3/deqp-gles3"),
		"gles31": filepath.Join(deqpBaseDir, "modules/gles31/deqp-gles31"),
		"vk":     filepath.Join(deqpBaseDir, "external/vulkancts/modules/vulkan/deqp-vk"),
	}[api]
	return p, ok
}
