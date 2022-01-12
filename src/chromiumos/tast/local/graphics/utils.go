// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// The common path prefix for DEQP executables.
	deqpBaseDir = "/usr/local/deqp"

	// Path to the USE flags used by ChromiumCommandBuilder.
	uiUseFlagsPath = "/etc/ui_use_flags.txt"

	// Path to get/set the dirty_writeback_centisecs kernel parameter.
	dirtyWritebackCentisecsPath = "/proc/sys/vm/dirty_writeback_centisecs"

	// Path to get/set the hangcheck_period_ms kernel parameter.
	hangCheckPeriodPath = "/sys/kernel/debug/dri/*/hangcheck_period_ms"
)

// APIType identifies a graphics API that can be tested by DEQP.
type APIType int

const (
	// UnknownAPI represents an unknown API.
	UnknownAPI APIType = iota
	// EGL represents Khronos EGL.
	EGL
	// GLES2 represents OpenGL ES 2.0.
	GLES2
	// GLES3 represents OpenGL ES 3.0.
	GLES3
	// GLES31 represents OpenGL ES 3.1. Note that DEQP also includes OpenGL ES
	// 3.2 in this category to reduce testing time.
	GLES31
	// VK represents Vulkan.
	VK
)

// Provided for getting readable API names.
func (a APIType) String() string {
	switch a {
	case EGL:
		return "EGL"
	case GLES2:
		return "GLES2"
	case GLES3:
		return "GLES3"
	case GLES31:
		return "GLES31"
	case VK:
		return "VK"
	}
	return fmt.Sprintf("UNKNOWN (%d)", a)
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
func extractOpenGLVersion(ctx context.Context, wflout string) (major,
	minor int, err error) {
	re := regexp.MustCompile(
		`OpenGL version string: OpenGL ES ([0-9]+).([0-9]+)`)
	matches := re.FindAllStringSubmatch(wflout, -1)
	if len(matches) != 1 {
		if dir, ok := testing.ContextOutDir(ctx); ok {
			if err := ioutil.WriteFile(filepath.Join(dir, "wflinfo.txt"),
				[]byte(wflout), 0644); err != nil {
				testing.ContextLog(ctx, "Failed to write wflinfo output: ", err)
			}
		}
		return 0, 0, errors.Errorf("%d OpenGL version strings found in wflinfo output", len(matches))
	}
	testing.ContextLogf(ctx, "Got %q", matches[0][0])
	if major, err = strconv.Atoi(matches[0][1]); err != nil {
		return 0, 0, errors.Wrap(err, "could not parse major version")
	}
	if minor, err = strconv.Atoi(matches[0][2]); err != nil {
		return 0, 0, errors.Wrap(err, "could not parse minor version")
	}
	return major, minor, nil
}

// GLESVersion returns the OpenGL major and minor versions extracted from the
// output of the wflinfo command. This is roughly a port of get_gles_version()
// defined in autotest/files/client/cros/graphics/graphics_utils.py.
func GLESVersion(ctx context.Context) (major, minor int, err error) {
	f, err := parseUIUseFlags(uiUseFlagsPath)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get UI USE flags")
	}
	cmd := testexec.CommandContext(ctx, "wflinfo", "-p", "null", "-a", api(f))
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return 0, 0, errors.Wrap(err, "running the wflinfo command failed")
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
			return false, errors.Wrap(err, "libvulkan.so search error")
		}
	}
	if !hasVulkan {
		testing.ContextLog(ctx, "Could not find libvulkan.so")
		return false, nil
	}

	// Then, search for the DEQP Vulkan testing binary.
	p, err := DEQPExecutable(VK)
	if err != nil {
		return false, errors.Wrapf(err, "could not get the executable for %v", VK)
	}
	if _, err := os.Stat(p); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, errors.Wrapf(err, "%v search error", p)
	}

	testing.ContextLogf(ctx, "Found libvulkan.so but not the %v binary", p)
	return false, nil
}

// SupportedAPIs returns an array of supported APIs given the OpenGL version and
// whether Vulkan is supported. If no APIs are supported, nil is returned. This
// is a port of part of the functionality of GraphicsApiHelper defined in
// autotest/files/client/cros/graphics/graphics_utils.py.
func SupportedAPIs(glMajor, glMinor int, vulkan bool) []APIType {
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
func DEQPExecutable(api APIType) (string, error) {
	switch api {
	case EGL:
		return "", errors.New("cannot run DEQP/EGL on Chrome OS")
	case GLES2:
		return filepath.Join(deqpBaseDir, "modules/gles2/deqp-gles2"), nil
	case GLES3:
		return filepath.Join(deqpBaseDir, "modules/gles3/deqp-gles3"), nil
	case GLES31:
		return filepath.Join(deqpBaseDir, "modules/gles31/deqp-gles31"), nil
	case VK:
		return filepath.Join(deqpBaseDir, "external/vulkancts/modules/vulkan/deqp-vk"), nil
	}
	return "", errors.Errorf("unknown graphics API: %s", api)
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

// SetDirtyWritebackDuration flushes pending data to disk and sets the
// dirty_writeback_centisecs kernel parameter to a specified time. If the time
// is negative, it only flushes pending data without changing the kernel
// parameter. This function should be used before starting graphics tests to
// shorten the time between flushes so that logs retain as much information as
// possible in case the system hangs/reboots. Note that the specified time is
// rounded down to the nearest centisecond. This is a port of the
// set_dirty_writeback_centisecs() function in
// autotest/files/client/bin/utils.py.
func SetDirtyWritebackDuration(ctx context.Context, d time.Duration) error {
	// Performing a full sync makes it less likely that there are pending writes
	// that might defer logging from being written immediately later.
	cmd := testexec.CommandContext(ctx, "sync")
	err := cmd.Run()
	if err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "sync failed")
	}
	if d >= 0 {
		centisecs := d / (time.Second / 100)
		if err = ioutil.WriteFile(dirtyWritebackCentisecsPath, []byte(fmt.Sprintf("%d", centisecs)), 0600); err != nil {
			return err
		}

		// Read back the kernel parameter because it is possible that the
		// written value was silently discarded (e.g. if the value was too
		// large).
		actual, err := GetDirtyWritebackDuration()
		if err != nil {
			return errors.Wrapf(err, "could not read %v", filepath.Base(dirtyWritebackCentisecsPath))
		}
		expected := centisecs * (time.Second / 100)
		if actual != expected {
			return errors.Errorf("%v contains %d after writing %d", filepath.Base(dirtyWritebackCentisecsPath), actual, expected)
		}
	}
	return nil
}

// GetDirtyWritebackDuration reads the dirty_writeback_centisecs kernel
// parameter and returns it as a time.Duration (in nanoseconds). Note that it is
// possible for the returned value to be negative. This is a port of the
// get_dirty_writeback_centisecs() function in
// autotest/files/client/bin/utils.py.
func GetDirtyWritebackDuration() (time.Duration, error) {
	b, err := ioutil.ReadFile(dirtyWritebackCentisecsPath)
	if err != nil {
		return -1, err
	}
	s := strings.TrimSpace(string(b))
	if len(s) == 0 {
		return -1, errors.Errorf("%v is empty", filepath.Base(dirtyWritebackCentisecsPath))
	}
	centisecs, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return -1, errors.Wrapf(err, "could not parse %v", filepath.Base(dirtyWritebackCentisecsPath))
	}
	return time.Duration(centisecs) * (time.Second / 100), nil
}

// SetHangCheckTimer sets the hangcheck timer to d to allow longer gpu runtime before hangcheck kicks in.
// If multiple hangcheck configuration paths are found, all of them are modified.
// Notice that it is expected to fail if running on older kernels.
// Notice that the unit of hangcheck timer is millisecond and the function would fail if d is smaller or equal to 1 millisecond.
func SetHangCheckTimer(ctx context.Context, d time.Duration) error {
	if d < 1*time.Millisecond {
		return errors.Errorf("invalid hangcheck timer parameter, %v, hangcheck timer must be greater or equal to 1 millisecond", d)
	}

	ps, err := filepath.Glob(hangCheckPeriodPath)
	if err != nil {
		return errors.Wrapf(err, "failed to list files with pattern: %v", hangCheckPeriodPath)
	}
	if len(ps) == 0 {
		return errors.Wrapf(err, "none of the path found with pattern: %v", hangCheckPeriodPath)
	}

	for _, p := range ps {
		periodMs := int64(d / time.Millisecond)
		if err := ioutil.WriteFile(p, []byte(fmt.Sprintf("%d", periodMs)), 0600); err != nil {
			return errors.Wrapf(err, "failed to write %d to %s", periodMs, p)
		}
		testing.ContextLogf(ctx, "Wrote %d to %s", periodMs, p)
	}
	return nil
}

// GetHangCheckTimer returns the current hangcheck duration timer.
// If multiple hangcheck configuration paths are found, it returns the smallest timer among all the configurations.
// Notice that it is expected to fail if running on older kernels and it is better to produce a warning instead of failing.
func GetHangCheckTimer() (time.Duration, error) {
	ps, err := filepath.Glob(hangCheckPeriodPath)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to list files with pattern: %v", hangCheckPeriodPath)
	}
	if len(ps) == 0 {
		return -1, errors.Wrapf(err, "none of the path found with pattern: %v", hangCheckPeriodPath)
	}

	minDuration := uint64(math.Inf(1))
	for _, p := range ps {
		b, err := ioutil.ReadFile(p)
		if err != nil {
			return -1, errors.Wrapf(err, "failed to read %s", p)
		}
		s := strings.TrimSpace(string(b))
		if len(s) == 0 {
			return -1, errors.Errorf("%s is empty", p)
		}
		d, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return -1, errors.Wrapf(err, "malformed content in %s: %s", p, s)
		}
		if d < minDuration {
			minDuration = d
		}
	}
	return time.Duration(minDuration) * time.Millisecond, nil
}
