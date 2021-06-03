// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UreadaheadValidation,
		Desc: "Validates that ARC ureadahead pack exists and looks valid",
		Contacts: []string{"khmel@google.com",
			"alanding@google.com",
			"arc-performance@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		// Minimum acceptable.
		Timeout: 4 * time.Minute,
	})
}

// ureadaheadPackRequired returns true in case ureadahead is required for the current device.
func ureadaheadPackRequired(ctx context.Context, isVMEnabled bool) (bool, error) {
	const (
		// please see arc.DataCollector for description.
		minVMMemoryKb = 7500000
	)

	if !isVMEnabled {
		// For container ureadahead pack is always required.
		return true, nil
	}

	memInfo, err := testexec.CommandContext(ctx, "cat", "/proc/meminfo").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to read /proc/meminfo")
	}

	memTotal := regexp.MustCompile(`(\n|^)MemTotal:\s+(\d+)\s+kB(\n|$)`).FindSubmatch(memInfo)
	if memTotal == nil {
		return false, errors.Errorf("required MemTotal is not found in %q", memInfo)
	}
	memTotalInt, err := strconv.Atoi(string(memTotal[2]))
	if err != nil || memTotalInt <= 0 {
		return false, errors.Errorf("failed to parse %q", memTotal[2])
	}

	return memTotalInt >= minVMMemoryKb, nil
}

func UreadaheadValidation(ctx context.Context, s *testing.State) {
	const (
		// normally generated ureadahead pack covers >350MB of data.
		minAcceptableUreadaheadPackizeKb = 300 * 1024

		// name of ureadahead log.
		ureadaheadLogName = "ureadahead.log"
	)

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to get whether ARCVM is enabled: ", err)
	}

	packPath := ""
	if isVMEnabled {
		// TODO(khmel): Add guest side validation.
		packPath = "/opt/google/vms/android/ureadahead.pack"
	} else {
		packPath = "/opt/google/containers/android/ureadahead.pack"
	}

	if _, err := os.Stat(packPath); err != nil {
		if os.IsNotExist(err) {
			required, err := ureadaheadPackRequired(ctx, isVMEnabled)
			if err != nil {
				s.Fatalf("Failed to check if ureadahead pack %s required, %v : ", packPath, err)
			}
			if required {
				s.Fatalf("ureadahead pack %s does not exist but required", packPath)
			} else {
				testing.ContextLogf(ctx, "ureadahead pack %s does not exist and is not required", packPath)
				return
			}
		} else {
			s.Fatalf("Failed to check ureadahead pack exists %s, %v : ", packPath, err)
		}
	}

	logPath := filepath.Join(s.OutDir(), ureadaheadLogName)
	cmd := testexec.CommandContext(ctx, "/sbin/ureadahead", "--dump", packPath)

	logFile, err := os.Create(logPath)
	if err != nil {
		s.Fatal("Failed to create log file: ", err)
	}
	cmd.Stdout = logFile
	// don't set cmd.Stderr, so it goes to default log buffer
	// and DumpLogOnError can dump it.
	err = cmd.Run(testexec.DumpLogOnError)
	logFile.Close()

	if err != nil {
		s.Fatalf("Failed to get the ureadahead stats %s, %v : ", packPath, err)
	}

	re := regexp.MustCompile(`^(\d+) inode groups, (\d+) files, (\d+) blocks \((\d+) kB\)$`)

	logFile, err = os.Open(logPath)
	if err != nil {
		s.Fatal("Failed to open log file: ", err)
	}
	defer logFile.Close()

	scanner := bufio.NewScanner(logFile)

	matchFound := false
	sizeKb := -1
	for scanner.Scan() {
		str := scanner.Text()
		result := re.FindStringSubmatch(str)
		if result == nil {
			continue
		}
		if matchFound {
			s.Fatalf("More than 1 match found. Please check %q. Last match: %q", ureadaheadLogName, str)
		}

		// Parsing (\d+) group that represents number of Kb handled by this ureadahead pack
		sizeKb, err = strconv.Atoi(result[4])
		if err != nil {
			s.Fatalf("Failed to parse group %q from %q, %v : ", result[4], str, err)
		}
		matchFound = true
	}

	if err := scanner.Err(); err != nil {
		s.Fatalf("Failed to read log file %q, %v", ureadaheadLogName, err)
	}

	if !matchFound {
		s.Fatalf("Failed to parse ureadahead pack dump. Please check %q", ureadaheadLogName)
	}

	if sizeKb < minAcceptableUreadaheadPackizeKb {
		s.Errorf("Pack size %d kB is too small. It is expected to be min %d kb", sizeKb, minAcceptableUreadaheadPackizeKb)
	}
}
