// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/platform/fsinfo"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ReportDiskUsage,
		Desc:     "Reports available disk space in the root filesystem",
		Contacts: []string{"norvez@chromium.org", "sarthakkukreti@chromium.org", "chromeos-storage@google.com"},
		// chromeos-assets is not available on devices without Chrome, require |chrome|
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
	})
}

func ReportDiskUsage(ctx context.Context, s *testing.State) {
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	// Report the production image size if it exists.
	const prodFile = "/root/bytes-rootfs-prod"
	if b, err := ioutil.ReadFile(prodFile); err == nil {
		if size, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64); err != nil {
			s.Errorf("Failed to parse %q from %v: %v", string(b), prodFile, err)
		} else {
			pv.Set(perf.Metric{
				Name:      "bytes_rootfs_prod",
				Unit:      "bytes",
				Direction: perf.SmallerIsBetter,
			}, float64(size))
		}
	}

	// Report the live size reported by df.
	if info, err := fsinfo.Get(ctx, "/"); err != nil {
		s.Error("Failed to get information about root filesystem: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "bytes_rootfs_test",
			Unit:      "bytes",
			Direction: perf.SmallerIsBetter,
		}, float64(info.Used))
	}

	// Report the size of specific directories that are particularly large.
	metrics := map[string]string{
		"/opt/":                       "bytes_opt",
		"/opt/google/chrome/":         "bytes_chrome",
		"/usr/bin":                    "bytes_bin",
		"/usr/sbin":                   "bytes_sbin",
		"/usr/share/chromeos-assets/": "bytes_assets",
		"/usr/share/chromeos-assets/input_methods/":    "bytes_ime",
		"/usr/share/chromeos-assets/speech_synthesis/": "bytes_tts",
		"/usr/share/fonts/":                            "bytes_fonts",
	}
	switch arch := runtime.GOARCH; arch {
	case "amd64":
		metrics["/usr/lib64/"] = "bytes_lib"
	case "arm64":
		metrics["/usr/lib64/"] = "bytes_lib"
	case "arm":
		metrics["/usr/lib/"] = "bytes_lib"
	default:
		s.Errorf("unsupported architecture %q", arch)
	}
	if arc.Supported() {
		if t, ok := arc.Type(); ok {
			switch t {
			case arc.Container:
				metrics[arc.ARCPath] = "bytes_arc"
			case arc.VM:
				metrics[arc.ARCVMPath] = "bytes_arc"
			default:
				s.Errorf("unsupported ARC type %d", t)
			}
		} else {
			s.Error("Failed to detect ARC type")
		}
	}

	// Find the space used in a directory, in bytes.
	//
	// This function uses 'du' to find the size of directories on a live DUT.
	// In practice this test will run on "test" images that are ever so
	// slightly different from the "base" image, so the results could be
	// slightly different from production images. However:
	// - the difference is tiny, overall size difference between base and
	// test images is <100KB
	// - if anything the size will be slightly larger on the test image, so
	// we're monitoring the worst case scenario anyway
	dirSize := func(path string) (int64, error) {
		cmd := testexec.CommandContext(ctx, "du", "-s", "-B1", "-x", path)
		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return 0, errors.Wrap(err, "du command failed")
		}
		// Parse the output from a "du -s -B1 -x <path>" command.
		//
		// The output is expected to have the following form:
		// 419024896	/opt/google/chrome/
		fields := strings.Fields(strings.TrimSpace(string(out)))
		if len(fields) != 2 {
			return 0, errors.Errorf("expected 2 fields in line %q", string(out))
		}
		size, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, errors.Errorf("failed to parse value %q", fields[0])
		}
		return size, nil
	}

	for k, v := range metrics {
		if size, err := dirSize(k); err != nil {
			s.Errorf("Failed to get the size of directory %q: %v", k, err)
		} else {
			pv.Set(perf.Metric{
				Name:      v,
				Unit:      "bytes",
				Direction: perf.SmallerIsBetter,
			}, float64(size))
		}
	}
}
