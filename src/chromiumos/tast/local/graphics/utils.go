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
const dirtyWritebackCentisecsPath = "/proc/sys/vm/dirty_writeback_centisecs"

// APIType identifies a graphics API.
type APIType int

const (
	// GLES2 represents OpenGL ES 2.0.
	GLES2 APIType = iota
	// GLES3 represents OpenGL ES 3.0.
	GLES3
	// GLES31 represents OpenGL ES 3.1.
	GLES31
	// VK represents Vulkan.
	VK
)

// Provided for getting readable API names in unit tests.
func (a APIType) String() string {
	switch a {
	case GLES2:
		return "gles2"
	case GLES3:
		return "gles3"
	case GLES31:
		return "gles31"
	case VK:
		return "vk"
	}
	return "unknown"
}

// parseUIUseFlags parses the configuration file located at path to get the UI
// USE flags: empty lines and lines starting with # are ignored. No end-of-line
// comments should be used. An empty non-nil map is returned if no flags are
// parsed. This is roughly a port of get_ui_use_flags() defined in
// autotest/files/client/bin/utils.py.
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
	for _, dir := range []string{"/usr/lib", "/usr/lib64", "/usr/local/lib", "/usr/local/lib64"} {
		if _, err := os.Stat(filepath.Join(dir, "libvulkan.so")); err == nil {
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

	// Then, search for the DEQP Vulkan testing binary.
	p := DEQPExecutable(VK)
	if len(p) == 0 {
		return false, fmt.Errorf("could not get the path for the %q API", VK)
	}
	if _, err := os.Stat(p); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("%v search error: %v", p, err)
	}

	testing.ContextLogf(ctx, "Found libvulkan.so but not the %v binary", p)
	return false, nil
}

// SupportedAPIs returns an array of supported APIs given the OpenGL version and
// whether Vulkan is supported. If no APIs are supported, nil is returned. This
// is a port of part of the functionality of GraphicsApiHelper defined in
// autotest/files/client/cros/graphics/graphics_utils.py.
func SupportedAPIs(glMajor int, glMinor int, vulkan bool) []APIType {
	var apis []APIType
	if glMajor >= 2 {
		apis = append(apis, GLES2)
	}
	if glMajor >= 3 {
		apis = append(apis, GLES3)
		if glMajor > 3 || glMinor >= 1 {
			apis = append(apis, GLES31)
		}
	}
	if vulkan {
		apis = append(apis, VK)
	}
	return apis
}

// DEQPExecutable maps an API identifier to the path of the appropriate DEQP
// executable (or an empty string if the API identifier is not valid). This is a
// port of part of the functionality of GraphicsApiHelper defined in
// autotest/files/client/cros/graphics/graphics_utils.py.
func DEQPExecutable(api APIType) string {
	switch api {
	case GLES2:
		return filepath.Join(deqpBaseDir, "modules/gles2/deqp-gles2")
	case GLES3:
		return filepath.Join(deqpBaseDir, "modules/gles3/deqp-gles3")
	case GLES31:
		return filepath.Join(deqpBaseDir, "modules/gles31/deqp-gles31")
	case VK:
		return filepath.Join(deqpBaseDir, "external/vulkancts/modules/vulkan/deqp-vk")
	}
	return ""
}

// DEQPEnvironment returns a list of environment variables of the form
// "key=value" that are appropriate for running DEQP binaries. To build it, the
// function starts from the given environment and modifies the LD_LIBRARY_PATH
// to insert /usr/local/lib:/usr/local/lib64 in the front, even if those two
// folders are already in the value. This is a port of part of the functionality
// of the initialization defined in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
func DEQPEnvironment(env []string) []string {
	// Start from a copy of the passed environment.
	nenv := make([]string, len(env))
	copy(nenv, env)

	// Search for the LD_LIBRARY_PATH variable in the environment.
	oldld := ""
	ldi := -1
	for i, s := range nenv {
		// Each s is of the form key=value.
		kv := strings.Split(s, "=")
		if kv[0] == "LD_LIBRARY_PATH" {
			ldi = i
			oldld = kv[1]
			break
		}
	}

	const paths = "/usr/local/lib:/usr/local/lib64"
	if ldi != -1 {
		// Found the LD_LIBRARY_PATH variable in the environment.
		if len(oldld) > 0 {
			nenv[ldi] = fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", paths, oldld)
		} else {
			nenv[ldi] = "LD_LIBRARY_PATH=" + paths
		}
	} else {
		// Did not find the LD_LIBRARY_PATH variable in the environment.
		nenv = append(nenv, "LD_LIBRARY_PATH="+paths)
	}

	return nenv
}

// SetDirtyWritebackCentisecs flushes pending data to disk and sets the
// dirty_writeback_centisecs kernel parameter to a specified time (in
// centiseconds). If the time is negative, it only flushes pending data without
// changing the kernel parameter. This is a port of the
// set_dirty_writeback_centisecs() function in
// autotest/files/client/bin/utils.py.
func SetDirtyWritebackCentisecs(ctx context.Context, centisecs int) error {
	// Flush buffers first to make this function synchronous.
	cmd := testexec.CommandContext(ctx, "sync")
	err := cmd.Run()
	if err != nil {
		cmd.DumpLog(ctx)
		return fmt.Errorf("running the sync command failed: %v", err)
	}
	if centisecs >= 0 {
		f, err := os.OpenFile(dirtyWritebackCentisecsPath, os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if _, err = f.WriteString(strconv.Itoa(centisecs)); err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// GetDirtyWritebackCentisecs reads the dirty_writeback_centisecs kernel
// parameter and returns it as an integer. This is a port of the
// get_dirty_writeback_centisecs() function in
// autotest/files/client/bin/utils.py.
func GetDirtyWritebackCentisecs() (int, error) {
	b, err := ioutil.ReadFile(dirtyWritebackCentisecsPath)
	if err != nil {
		return -1, err
	}
	if len(b) == 0 {
		return -1, fmt.Errorf("dirty_writeback_centisecs is empty")
	}
	centisecs, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return -1, fmt.Errorf("could not parse dirty_writeback_centisecs: %v", err)
	}
	return centisecs, nil
}
