// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"encoding/xml"
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

// DEQPOutcomeIsFailure decides if an outcome found in the output of a DEQP test
// is considered a failure. This is a port of the TEST_RESULT_FILTER list in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py.
func DEQPOutcomeIsFailure(s string) bool {
	var nonFail map[string]struct{} = map[string]struct{}{
		"pass": struct{}{},
		"notsupported": struct{}{},
		"internalerror": struct{}{},
		"qualitywarning": struct{}{},
        "compatibilitywarning": struct{}{},
        "skipped": struct{}{},
	}
	_, isNonFail := nonFail[strings.ToLower(s)]
	return !isNonFail
}

// ParseDEQPOutput parses the given DEQP log file to extract the number of tests
// per outcome (returned in the stats map) and the names of the tests that
// failed. An error is returned if an irrecoverable error occurs, i.e., an error
// that can suggest problems with the DEQP output.
//
// The returned stats map might look something like
//   "pass": 3
//   "fail": 1
//
// This means that 3 tests passed and 1 failed. A recoverable error is reported
// in the stats map with the reserved outcome "parsefailed", and the
// corresponding test name is added to the failed slice.
//
// This parser expects the format explained in
// https://android.googlesource.com/platform/external/deqp/+/deqp-dev/doc/qpa_file_format.txt
// but only cares about the #beginTestCaseResult ... #endTestCaseResult or
// #beginTestCaseResult ... #terminateTestCaseResult sections.
//
// This is a (hopefully improved) port of the functionality of the parsing
// function defined in
// autotest/files/client/site_tests/graphics_dEQP/graphics_dEQP.py: the version
// here tends to be more conservative with parsing errors since they could
// indicate a problem with the DEQP output.
//
// TODO(andrescj): we may also need to return the tests that didn't fail so that
// the caller can decide if there are missing tests. It seems that DEQP ignores
// other tests once one of them is killed by the watchdog. In this case, those
// ignored tests don't get added to the failed slice.
func ParseDEQPOutput(p string) (stats map[string]uint, failed []string, err error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, nil, err
	}

	// State to detect when XML code is about to start.
	start := false

	// To accumulate statistics.
	stats = make(map[string]uint)

	// To accumulate XML code inside the sections we care about.
	var rawXML strings.Builder

	// To hold the test case name when we get there.
	var test string

	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		// Flag to detect when the XML is expected to be complete.
		complete := false

		// Flag to detect a recoverable parsing error.
		bad := false

		// To hold the outcome of the test case when we get it.
		var outcome string

		if strings.HasPrefix(l, "#terminateTestCaseResult") {
			if !start {
				// We shouldn't see a terminate without a begin.
				return nil, nil, fmt.Errorf("unexpected #terminateTestCaseResult on line %v", i+1)
			}
			// If the test terminates early, the XML could be incomplete and
			// should not be parsed. Get the cause for early termination.
			bad = true
			outcome = "parsefailed"
			if s := strings.TrimSpace(strings.TrimPrefix(l, "#terminateTestCaseResult")); len(s) > 0 {
				outcome = s
			} else {
				// #terminateTestCaseResult is not accompanied by a cause. If
				// this is the last line, let's assume that a Tast timeout
				// occurred and make this error recoverable. Otherwise, report
				// an irrecoverable error.
				if i < len(lines) - 1 {
					return nil, nil, fmt.Errorf("missing cause for #terminateTestCaseResult on line %v", i+1)
				}
			}
		} else if strings.HasPrefix(l, "#endTestCaseResult") {
			if !start {
				// We shouldn't see an end without a begin.
				return nil, nil, fmt.Errorf("unexpected #endTestCaseResult on line %v", i+1)
			}
			complete = true
		} else if strings.HasPrefix(l, "#beginTestCaseResult") {
			if start {
				// If we see another begin before an end/terminate then
				// something is wrong.
				return nil, nil, fmt.Errorf("unexpected #beginTestCaseResult on line %v", i+1)
			} else {
				// Derive the test name from #beginTestCaseResult.
				if test = strings.TrimSpace(strings.TrimPrefix(l, "#beginTestCaseResult")); len(test) == 0 {
					return nil, nil, fmt.Errorf("#beginTestCaseResult is not followed by test name on line %v", i+1)
				}
				start = true
			}
		} else if start {
			// We don't need to add a newline: the XML parser is ok with that.
			rawXML.WriteString(l)
		}

		if complete || bad {
			if complete {
				// Structure to parse the XML into. Note that it's necessary to
				// capitalize the first letter of each field so that
				// xml.Unmarshal works.
				r := struct {
					XMLName xml.Name `xml:"TestCaseResult"`
					CasePath string `xml:",attr"`
					Result []struct {
						StatusCode string `xml:",attr"`
					}
				}{}

				// Parse and perform sanity checks.
				if err := xml.Unmarshal([]byte(rawXML.String()), &r); err != nil {
					return nil, nil, fmt.Errorf("could not parse XML for %v: %v", test, err)
				}
				if len(r.CasePath) == 0 || r.CasePath != test {
					return nil, nil, fmt.Errorf("bad CasePath attribute for %v: %q", test, r.CasePath)
				}
				if len(r.Result) != 1 {
					return nil, nil, fmt.Errorf("%v <Result> elements found for %v", len(r.Result), test)
				}
				outcome = strings.TrimSpace(r.Result[0].StatusCode)
				if len(outcome) == 0 {
					return nil, nil, fmt.Errorf("bad StatusCode attribute for %v: %q", test, outcome)
				}
			}

			if DEQPOutcomeIsFailure(outcome) {
				failed = append(failed, test)
			}

			// Get ready for another test case.
			stats[strings.ToLower(outcome)]++
			outcome = ""
			rawXML.Reset()
			start = false
			complete = false
			bad = false
		}
	}

	// If start = true, the input for the last test is incomplete (maybe Tast
	// timed out or a crash occurred). We can make the error here recoverable so
	// that the other parsed tests can still be used.
	if start {
		failed = append(failed, test)
		stats[strings.ToLower("parsefailed")]++
	}
	return stats, failed, nil
}