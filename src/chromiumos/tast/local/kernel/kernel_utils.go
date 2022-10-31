// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernel contains kernel-related utility functions for local tests.
package kernel

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

var (
	archFields        = make(map[string]map[string][]string)
	amdgpuFields      = make(map[string][]string)
	armFields         = make(map[string][]string)
	cirrusFields      = make(map[string][]string)
	exynosFields      = make(map[string][]string)
	i915Fields        = make(map[string][]string)
	i915FieldsKernel5 = make(map[string][]string)
	mediatekFields    = make(map[string][]string)
	qualcommFields    = make(map[string][]string)
	rockchipFields    = make(map[string][]string)
	tegraFields       = make(map[string][]string)
	virtioFields      = make(map[string][]string)
)

// contains checks if an element of type string exists in a slice of strings.
func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

// ReadKernelConfig reads the kernel config key value pairs trimming CONFIG_ prefix from the keys.
func ReadKernelConfig(ctx context.Context) (map[string]string, error) {
	configs, err := readKernelConfigBytes(ctx)
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)

	for _, line := range strings.Split(string(configs), "\n") {
		line := strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) < 2 || kv[1] == "" {
			return nil, errors.Errorf("unexpected config line %q", line)
		}
		const configPrefix = "CONFIG_"
		if !strings.HasPrefix(kv[0], configPrefix) {
			return nil, errors.Errorf("config %q doesn't start with %s unexpectedly", kv[0], configPrefix)
		}
		res[strings.TrimPrefix(kv[0], configPrefix)] = kv[1]
	}
	return res, nil
}

// readKernelConfigBytes reads the kernel config bytes
func readKernelConfigBytes(ctx context.Context) ([]byte, error) {
	const filename = "/proc/config.gz"
	// Load configs module to generate /proc/config.gz.
	if err := testexec.CommandContext(ctx, "modprobe", "configs").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to generate kernel config file")
	}
	var r io.ReadCloser
	f, err := os.Open(filename)
	if err != nil {
		testing.ContextLogf(ctx, "Falling back: failed to open %s: %v", filename, err)
		u, err := sysutil.Uname()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get uname")
		}
		fallbackFile := "/boot/config-" + u.Release
		r, err = os.Open(fallbackFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %s", fallbackFile)
		}
	} else { // Normal path.
		defer f.Close()
		r, err = gzip.NewReader(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create gzip reader for %s", filename)
		}
	}
	defer r.Close()
	configs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}
	return configs, nil
}

// getMeminfo reads memory usage from /proc/meminfo
func getMeminfo(ctx context.Context, tags []string) ([]int, error) {
	var out []int
	for _, tag := range tags {
		stdout, _, err := testexec.CommandContext(ctx, "grep", tag, "/proc/meminfo").SeparatedOutput(testexec.DumpLogOnError)
		if err != nil {
			return out, errors.Wrapf(err, "could not grep %s from /proc/meminfo", tag)
		}
		memoryUsage := regexp.MustCompile("\\d+")
		extracted := memoryUsage.FindAllString(string(stdout), -1)
		val, err := strconv.Atoi(extracted[0])
		if err != nil {
			return out, errors.Wrapf(err, "error occured while converting string %s to int", extracted)
		}
		out = append(out, val)
	}
	return out, nil
}

// listGrep returns true iff any item in list matches the specified pattern.
func listGrep(list, pattern string) bool {

	compiled := regexp.MustCompile(pattern)
	match := compiled.FindAllStringSubmatch(list, -1)
	return len(match) > 0
}

// getArmSocFamilyFromDeviceTree works out which ARM SoC we're running on based on the 'compatible' property
func getArmSocFamilyFromDeviceTree() string {

	compatible, err := os.ReadFile("/sys/firmware/devicetree/base/compatible")
	if err != nil {
		return ""
	}
	if listGrep(string(compatible), "^rockchip,") {
		return "rockchip"
	}
	if listGrep(string(compatible), "^mediatek,") {
		return "mediatek"
	}
	if listGrep(string(compatible), "^qcom,") {
		return "qualcomm"
	}
	return ""
}

// getArmSocFamily works out which ARM SoC we're running on
func getArmSocFamily(cpuinfo string) string {

	family := getArmSocFamilyFromDeviceTree()
	if len(family) > 0 {
		return family
	}
	if listGrep(cpuinfo, "EXYNOS5") {
		return "exynos5"
	}
	if listGrep(cpuinfo, "Tegra") {
		return "tegra"
	}
	if listGrep(cpuinfo, "Rockchip") {
		return "rockchip"
	}
	return "arm"
}

// getCPUArch works out which CPU architecture we're running on
func getCPUArch(ctx context.Context) (string, error) {
	stdout, stderr, err := testexec.CommandContext(ctx, "uname", "-m").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "could not get cpu architecture. %s", stderr)
	}
	if strings.HasPrefix(string(stdout), "aarch64") ||
		strings.HasPrefix(string(stdout), "arm") {
		return "arm", nil
	}
	if strings.HasPrefix(string(stdout), "x86_64") {
		return "x86_64", nil
	}
	if strings.HasPrefix(string(stdout), "i386") {
		return "i386", nil
	}
	return "", errors.Wrapf(nil, "unsupported machine type : %s", stdout)
}

// getCPUSocFamily works like getCPUArch, but for ARM, returns the SoC family name
func getCPUSocFamily(ctx context.Context) (string, error) {
	cpuinfo, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", errors.Wrap(err, "could not load cpuinfo")
	}
	family, err := getCPUArch(ctx)
	if family == "arm" {
		family = getArmSocFamily(string(cpuinfo))
	}
	if listGrep(string(cpuinfo), "^vendor_id.*:.*AMD") {
		family = "amd"
	}
	return family, err

}

// parseSysfsMemory parses output of graphics memory sysfs to determine the number of buffer objects and bytes.
func parseSysfsMemory(output string) (map[string]int, error) {
	results := make(map[string]int)
	labels := []string{"objects", "bytes"}
	var prevWord string
	/* First handle i915_gem_objects in 5.x kernels. Example:
	 *  296 shrinkable [0 free] objects, 274833408 bytes
	 * frecon: 3 objects, 72192000 bytes (0 active, 0 inactive, 0 unbound, 0 closed)
	 * chrome: 6 objects, 74629120 bytes (0 active, 0 inactive, 376832 unbound, 0 closed)
	 * <snip>
	 */
	i915MemoryRe := regexp.MustCompile("(?P<objects>\\d*) shrinkable.*objects, (?P<bytes>\\d*) bytes")
	matches := i915MemoryRe.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		groupNames := i915MemoryRe.SubexpNames()
		// the 0th index has the whole string so skip it.
		for index := 1; index < len(matches[0]); index++ {
			value, err := strconv.Atoi(matches[0][index])
			if err != nil {
				return nil, errors.Wrapf(err, " unable to convert value %s to int ", matches[0][index])
			}
			results[groupNames[index]] = value
		}
		return results, nil
	}
	for _, line := range strings.Split(output, "\n") {
		lineWords := strings.Split(strings.Replace(line, ",", "", -1), " ")
		for _, word := range lineWords {
			if _, exists := results[word]; !exists &&
				contains(labels, word) && len(prevWord) > 0 {
				value, err := strconv.Atoi(prevWord)
				if err != nil {
					return nil, errors.Wrapf(err, " unable to convert value %s to int ", value)
				}
				results[word] = value
			}
			prevWord = word
			if len(results) == len(labels) {
				return results, nil
			}
		}
	}
	return results, nil
}

func processMemErrors(ctx context.Context) (int, error) {
	numErrors := 0
	var soc, errMsg string
	var pathOut []byte
	var newErr error
	soc, err := getCPUSocFamily(ctx)
	if err != nil {
		return numErrors + 1, err
	}
	arch, err := getCPUArch(ctx)
	if err != nil {
		return numErrors + 1, err
	}
	if arch == "x86_64" || arch == "i386" {
		pciVgaOut, stderr, err := testexec.CommandContext(ctx, "lspci").SeparatedOutput(testexec.DumpLogOnError)
		if err != nil {
			return numErrors + 1, errors.Wrapf(newErr, "could not run lspci command : %s", string(stderr))
		}
		reVga := regexp.MustCompile("VGA.+")
		pciVgaDevice := string(reVga.FindString(string(pciVgaOut)))
		if strings.Contains(pciVgaDevice, "Advanced Micro Devices") {
			soc = "amdgpu"
		} else if strings.Contains(pciVgaDevice, "Intel Corporation") {
			soc = "i915"
			kernelVer, _, _ := sysutil.KernelVersionAndArch()
			if kernelVer.IsOrLater(4, 19) {
				soc = "i915_kernel_5"
			}
		} else if strings.Contains(pciVgaDevice, "Cirrus Logic") {
			soc = "cirrus"
		} else {
			lshwOut, stderr, err := testexec.CommandContext(ctx, "lshw", "-c", "video").SeparatedOutput(testexec.DumpLogOnError)
			if err != nil {
				return numErrors + 1, errors.Wrapf(newErr, "could not run lshw command : %s", string(stderr))
			}
			reVirtio := regexp.MustCompile("configuration:.*driver=.+")
			virtioMatch := string(reVirtio.FindString(string(lshwOut)))
			if strings.Contains(virtioMatch, "virtio") {
				soc = "virtio"
			}
		}
	}
	if _, exists := archFields[soc]; !exists {
		return numErrors + 1, errors.Wrapf(newErr, "Error: Architecture %s not yet supported", arch)
	}
	for _, fieldName := range archFields[soc] {
		for _, file := range fieldName {
			if _, err := os.Stat(file); err == nil {
				currPath, stderr, err := testexec.CommandContext(ctx, "cat", file).SeparatedOutput(testexec.DumpLogOnError)
				if err != nil {
					numErrors++
					errMsg = strings.Join([]string{errMsg, "Unable to find any sysfs paths for path :",
						file, string(stderr)}, " ")
				} else {
					pathOut = currPath
					break
				}
			}
			if len(string(pathOut)) == 0 {
				return numErrors + 1, errors.Wrapf(newErr, "could not find sysfs path for field %s", fieldName)
			}
			parsedResults, err := parseSysfsMemory(string(pathOut))
			if err != nil {
				return numErrors + 1, err
			}
			if bytes, exists := parsedResults["bytes"]; exists && bytes == 0 {
				numErrors++
				errMsg = strings.Join([]string{errMsg, file, "reported 0 bytes"}, " ")
			}
		}
	}
	// Make sure we can access memory in /proc/meminfo.
	_, err := getMeminfo(ctx, []string{"MemTotal", "MemFree", "SwapTotal", "SwapFree"})
	if err != nil {
		return numErrors + 1, err
	}
	// If an error was captured at any point return it.
	if len(errMsg) > 0 {
		return numErrors, errors.Wrap(err, errMsg)
	}
	return numErrors, nil
}

// GetMemErrors returns the number of errors encountered while gather memory usage metrics
func GetMemErrors(ctx context.Context) (int, error) {
	amdgpuFields["gem_objects"] = append(amdgpuFields["gem_objects"], "/sys/kernel/debug/dri/0/amdgpu_gem_info")
	amdgpuFields["memory"] = append(amdgpuFields["memory"], "/sys/kernel/debug/dri/0/amdgpu_gtt_mm")
	exynosFields["gem_objects"] = append(exynosFields["gem_objects"], "/sys/kernel/debug/dri/?/exynos_gem_objects")
	exynosFields["memory"] = append(exynosFields["memory"], "/sys/class/misc/mali0/device/memory")
	i915Fields["gem_objects"] = append(i915Fields["gem_objects"], "/sys/kernel/debug/dri/0/i915_gem_objects")
	i915Fields["memory"] = append(i915Fields["memory"], "/sys/kernel/debug/dri/0/i915_gem_gtt")
	i915FieldsKernel5["gem_objects"] = append(i915FieldsKernel5["gem_objects"], "/sys/kernel/debug/dri/0/i915_gem_objects")
	tegraFields["memory"] = append(tegraFields["memory"], "/sys/kernel/debug/memblock/memory")
	archFields["amdgpu"] = amdgpuFields
	archFields["exynos"] = exynosFields
	archFields["i915"] = i915Fields
	archFields["i915_kernel_5"] = i915FieldsKernel5
	archFields["qualcomm"] = qualcommFields
	archFields["rockchip"] = rockchipFields
	archFields["tegra"] = tegraFields
	return processMemErrors(ctx)
}
