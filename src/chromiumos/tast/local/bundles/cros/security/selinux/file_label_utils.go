// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	selinux "github.com/opencontainers/selinux/go-selinux"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// FilterResult is returned by a FileLabelCheckFilter indicating how a file
// should be handled.
type FilterResult int

const (
	// Skip indicates that the file should be skipped.
	Skip FilterResult = iota

	// Check indicates that the file's SELinux context should be checked.
	Check
)

// FileLabelCheckFilter returns true if the file described by path
// and fi should be skipped. fi is never nil.
type FileLabelCheckFilter func(path string, fi os.FileInfo) (skipFile, skipSubdir FilterResult)

// IgnorePaths returns a FileLabelCheckFilter which allows the test to skip files
// or directories matching pathsToIgnore, including its subdirectory.
func IgnorePaths(pathsToIgnore []string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (FilterResult, FilterResult) {
		for _, path := range pathsToIgnore {
			if p == path {
				return Skip, Skip
			}
		}
		return Check, Check
	}
}

// IgnorePathsRegex returns a FileLabelCheckFilter which allows the test to
// skip files or directories matching pathsToIgnore, including its
// subdirectory.
func IgnorePathsRegex(pathsToIgnore []string) FileLabelCheckFilter {
	var compiled []*regexp.Regexp
	for _, path := range pathsToIgnore {
		compiled = append(compiled, regexp.MustCompile(fmt.Sprintf("^%s$", path)))
	}
	return func(p string, _ os.FileInfo) (FilterResult, FilterResult) {
		for _, pattern := range compiled {
			if pattern.MatchString(p) {
				return Skip, Skip
			}
		}
		return Check, Check
	}
}

// IgnorePathsButNotContents returns a FileLabelCheckFilter which allows the test
// to skip files matching pathsToIgnore, but not its subdirectory.
func IgnorePathsButNotContents(pathsToIgnore []string) FileLabelCheckFilter {
	return func(p string, _ os.FileInfo) (FilterResult, FilterResult) {
		for _, path := range pathsToIgnore {
			if p == path {
				return Skip, Check
			}
		}
		return Check, Check
	}
}

// IgnorePathButNotContents returns a FileLabelCheckFilter which allows the test
// to skip files matching pathsToIgnore, but not its subdirectory.
func IgnorePathButNotContents(pathToIgnore string) FileLabelCheckFilter {
	return IgnorePathsButNotContents([]string{pathToIgnore})
}

// CheckAll returns (Check, Check) to let the test to check all files
func CheckAll(_ string, _ os.FileInfo) (FilterResult, FilterResult) { return Check, Check }

// InvertFilterSkipFile takes one filter and return a FileLabelCheckFilter which
// reverses the boolean value for skipFile.
func InvertFilterSkipFile(filter FileLabelCheckFilter) FileLabelCheckFilter {
	return func(p string, fi os.FileInfo) (FilterResult, FilterResult) {
		skipFile, skipSubdir := filter(p, fi)
		if skipFile == Skip {
			return Check, skipSubdir
		}
		return Skip, skipSubdir
	}
}

// contextUnmatchError is returned by checkFileContext if the file context did
// not match with the expectation.
type contextUnmatchError struct {
	*errors.E
}

// checkFileContext takes a path and a expected, and return an error
// if the context mismatch or unable to check context.
func checkFileContext(ctx context.Context, path string, expected *regexp.Regexp, log bool, passIfNotFound bool) error {
	actual, err := selinux.FileLabel(path)
	if err != nil {
		if passIfNotFound && os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to get file context")
	}
	if !expected.MatchString(actual) {
		return &contextUnmatchError{E: errors.Errorf("got %q; want %q", actual, expected)}
	}
	if log {
		testing.ContextLogf(ctx, "File %q has correct label %q", path, actual)
	}
	return nil
}

// CheckContextReq holds parameters given to CheckContext.
type CheckContextReq struct {
	// Path is a file path to check.
	Path string

	// Expected is a regexp that should match with the SELinux context of files.
	Expected *regexp.Regexp

	// Recursive indicates whether to check child files recursively.
	Recursive bool

	// Filter is a function to filter files to check. It may not be nil.
	Filter FileLabelCheckFilter

	// IgnoreErrors indicates whether system call errors for Path should be
	// ignored. If Recursive is true, IgnoreError is set to true for all child
	// files recursively checked. This behavior is intentional to avoid typical
	// race conditions on special file systems (like sysfs and procfs).
	//
	// IgnoreErrors ignores all errors, not only "harmless" ones like ENOENT and
	// ENOTDIR. When accessing files in special file systems, they can return
	// arbitrary error code such as EIO. It does not make sense to make SELinux
	// tests fail by such errors since they are not directly related to what we
	// want to test.
	IgnoreErrors bool

	// Log indicates whether to log successful checks.
	Log bool
}

// CheckContext checks path to have selinux label match expected. Errors are
// passed through s.
func CheckContext(ctx context.Context, s *testing.State, req *CheckContextReq) {
	fi, err := os.Lstat(req.Path)
	if err != nil {
		if !req.IgnoreErrors {
			s.Errorf("Failed to stat %v: %v", req.Path, err)
		}
		return
	}

	skipFile, skipSubdir := req.Filter(req.Path, fi)

	if skipFile == Check {
		if err := checkFileContext(ctx, req.Path, req.Expected, req.Log, false); err != nil {
			if _, ok := err.(*contextUnmatchError); ok || !req.IgnoreErrors {
				s.Errorf("Failed file context check for %v: %v", req.Path, err)
			}
		}
	}

	if !fi.IsDir() || !req.Recursive || skipSubdir == Skip {
		return
	}

	fis, err := ioutil.ReadDir(req.Path)
	if err != nil {
		if !req.IgnoreErrors {
			s.Errorf("Failed to list directory %s: %s", req.Path, err)
		}
		return
	}
	for _, fi := range fis {
		CheckContext(ctx, s, &CheckContextReq{
			Path:         filepath.Join(req.Path, fi.Name()),
			Expected:     req.Expected,
			Recursive:    req.Recursive,
			Filter:       req.Filter,
			IgnoreErrors: true, // always ignore errors for child files
			Log:          req.Log,
		})
	}
}

// FileContextRegexp returns a regex to wrap given context with "^u:object_r:xxx:s0$".
func FileContextRegexp(context string) (*regexp.Regexp, error) {
	return regexp.Compile("^u:object_r:" + context + ":s0$")
}

// GpuDevices returns the folder for gpuDevices, for testcases for non-sysfs
// files.
func GpuDevices() ([]string, error) {
	var devices []string
	renderDs, err := filepath.Glob("/sys/class/drm/renderD*")
	if err != nil {
		return devices, errors.Wrap(err, "unable to locate render devices")
	}
	var firstErr error
	var errCnt int
	for _, entryTree := range renderDs {
		deviceReal, err := filepath.EvalSymlinks(filepath.Join(entryTree, "device"))
		if err != nil {
			if firstErr != nil {
				firstErr = errors.Wrap(err, "unable to resolve absolute deviceReal")
			}
			errCnt++
			continue
		}
		// entryTree may link to something looks like
		// ../../devices/pci0000:00/0000:00:02.0/virtio0/drm/renderD128
		// We only want the real device path.
		deviceReal = strings.SplitN(deviceReal, "/virtio", 2)[0]
		devices = append(devices, deviceReal)
	}
	if firstErr == nil {
		return devices, nil
	}
	return devices, errors.Wrapf(firstErr, "%d errors have occurred, first error is:", errCnt)
}

// IIOSensorDevices returns the folder for cros-ec related iio devices. even
// with err, devices without errors are still returned.
func IIOSensorDevices() ([]string, error) {
	var devices []string
	trees, err := filepath.Glob("/sys/bus/iio/devices/iio:device*")
	if err != nil {
		return devices, errors.Wrap(err, "unable to locate iio devices")
	}
	var firstErr error
	var errCnt int
	for _, entry := range trees {
		name, err := ioutil.ReadFile(filepath.Join(entry, "name"))
		if err != nil {
			if firstErr != nil {
				firstErr = errors.Wrap(err, "unable to determine device name")
			}
			errCnt++
			continue
		}
		deviceReal, err := filepath.EvalSymlinks(entry)
		if err != nil {
			if firstErr != nil {
				firstErr = errors.Wrap(err, "failed to evaluate symlink for iio device")
			}
			errCnt++
			continue
		}
		if strings.HasPrefix(string(name), "cros-ec") {
			devices = append(devices, deviceReal)
		}
	}
	if firstErr == nil {
		return devices, nil
	}
	return devices, errors.Wrapf(firstErr, "%d errors have occurred, first error is:", errCnt)
}

// IIOSensorFilter returns pairs of FilterResult to check only files that
// should have cros_sensor_hal_sysfs labeled.
func IIOSensorFilter(p string, fi os.FileInfo) (skipFile, skipSubdir FilterResult) {
	sensorFiles := map[string]bool{
		"flush":                               true,
		"frequency":                           true,
		"sampling_frequency":                  true,
		"in_activity_still_change_falling_en": true,
	}
	ringFiles := map[string]bool{
		"enable":          true,
		"length":          true,
		"current_trigger": true,
	}
	if sensorFiles[fi.Name()] {
		return Check, Check
	}
	if strings.Contains(p, "cros-ec-ring") && ringFiles[fi.Name()] {
		return Check, Check
	}
	return Skip, Check
}
