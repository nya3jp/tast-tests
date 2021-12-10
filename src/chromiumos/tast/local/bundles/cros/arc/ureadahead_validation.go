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

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UreadaheadValidation,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Validates that ARC ureadahead packs in the host/guest OS exist and are valid",
		Contacts: []string{
			"khmel@google.com",
			"alanding@google.com",
			"arc-performance@google.com",
		},
		// NOTE: This test should never be promoted to critical. It has build dependency and it will
		//       always fail in PFQ since we don't have ureadahead caches generated at PFQ time.
		Attr: []string{"group:mainline", "informational", "group:arc-functional"},
		// Skip userdebug boards which we don't generate ureadahead packs for.
		SoftwareDeps: []string{"chrome", "no_arc_userdebug"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		// Minimum acceptable.
		Timeout: 5 * time.Minute,
	})
}

// ureadaheadPackRequired returns true in case ureadahead is required for the current device.
func ureadaheadPackRequired(ctx context.Context, vmEnabled bool) (bool, error) {
	// please see arc.DataCollector for description.
	const minVMMemoryKB = 7500000

	if !vmEnabled {
		// For container ureadahead pack is always required.
		return true, nil
	}

	m, err := mem.VirtualMemory()
	if err != nil {
		return false, errors.Wrap(err, "failed to get memory info")
	}

	return m.Total > uint64(minVMMemoryKB*1024), nil
}

func UreadaheadValidation(ctx context.Context, s *testing.State) {
	const (
		// Names of ureadahead dump logs.
		ureadaheadLogName   = "ureadahead.log"
		ureadaheadVMLogName = "vm_ureadahead.log"
	)

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to get whether ARCVM is enabled: ", err)
	}

	packPath := ""
	if vmEnabled {
		packPath = "/opt/google/vms/android/ureadahead.pack"
	} else {
		packPath = "/opt/google/containers/android/ureadahead.pack"
	}

	if _, err := os.Stat(packPath); err != nil {
		if !os.IsNotExist(err) {
			s.Fatalf("Failed to check ureadahead pack exists %s: %v", packPath, err)
		}
		required, err := ureadaheadPackRequired(ctx, vmEnabled)
		if err != nil {
			s.Fatalf("Failed to check if ureadahead pack %s required: %v", packPath, err)
		}
		if required {
			s.Fatalf("ureadahead pack %s does not exist but required", packPath)
		}
		testing.ContextLogf(ctx, "ureadahead pack %s does not exist and is not required", packPath)
		return
	}

	logPath := filepath.Join(s.OutDir(), ureadaheadLogName)
	cmd := testexec.CommandContext(ctx, "/sbin/ureadahead", "--dump", packPath)

	logFile, err := os.Create(logPath)
	if err != nil {
		s.Fatal("Failed to create log file: ", err)
	}
	cmd.Stdout = logFile
	// Don't set cmd.Stderr, so it goes to default log buffer
	// and DumpLogOnError can dump it.
	err = cmd.Run(testexec.DumpLogOnError)
	logFile.Close()

	if err != nil {
		s.Fatalf("Failed to get the ureadahead stats %s: %v", packPath, err)
	}

	// Verify the host pack file dump.
	logFile, err = os.Open(logPath)
	if err != nil {
		s.Fatal("Failed to open log file: ", err)
	}
	defer logFile.Close()
	if err = checkPackFileDump(ctx, logPath); err != nil {
		s.Fatalf("Failed to verify ureadahead pack file dump, please check %q: %v", ureadaheadLogName, err)
	}

	// If VM, also verify guest OS ureadahead dump.
	if vmEnabled {
		vmLogPath := filepath.Join(s.OutDir(), ureadaheadVMLogName)
		if err = dumpGuestPack(ctx, vmLogPath); err != nil {
			s.Fatal("Failed to dump ureadahead pack in the guest OS: ", err)
		}

		// Verify the guest pack file dump.
		if err = checkPackFileDump(ctx, vmLogPath); err != nil {
			s.Fatalf("Failed to verify ureadahead pack file dump, please check %q: %v", ureadaheadVMLogName, err)
		}
	}
}

// dumpGuestPack dumps the ureadahead pack in the guest OS.
func dumpGuestPack(ctx context.Context, logPath string) error {
	// File path for ureadahead pack in the quest OS.
	const ureadaheadDataDir = "/var/lib/ureadahead"

	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get name of the output directory")
	}

	// Connect to ARCVM instance.
	a, err := arc.New(ctx, outdir)
	if err != nil {
		return errors.Wrap(err, "failed to connect to ARCVM")
	}
	defer a.Close(ctx)

	// Check for existence of newly generated pack file on guest side.
	srcPath := filepath.Join(ureadaheadDataDir, "pack")
	if _, err := a.FileSize(ctx, srcPath); err != nil {
		return errors.Wrap(err, "failed to ensure pack file exists")
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer logFile.Close()

	// Capture stdout into log file.
	cmd := a.Command(ctx, "/system/bin/ureadahead", "--dump")
	cmd.Stdout = logFile
	return cmd.Run(testexec.DumpLogOnError)
}

// checkPackFileDump verifies the validity of the generated pack file using
// ureadahead's own pack file dump functionality.
func checkPackFileDump(ctx context.Context, logPath string) error {
	// normally generated ureadahead pack covers >300MB of data.
	const minAcceptableUreadaheadPackSizeKB = 300 * 1024

	logFile, err := os.Open(logPath)
	if err != nil {
		return errors.Wrap(err, "failed to open log file")
	}
	defer logFile.Close()

	re := regexp.MustCompile(`^(\d+) inode groups, (\d+) files, (\d+) blocks \((\d+) kB\)$`)
	scanner := bufio.NewScanner(logFile)

	matchFound := false
	sizeKB := -1
	for scanner.Scan() {
		str := scanner.Text()
		result := re.FindStringSubmatch(str)
		if result == nil {
			continue
		}
		if matchFound {
			return errors.Wrapf(err, "failed with more than 1 match found. Last match: %q", str)
		}

		// Parsing (\d+) group that represents number of Kb handled by this ureadahead pack
		sizeKB, err = strconv.Atoi(result[4])
		if err != nil {
			return errors.Wrapf(err, "failed to parse group %q from %q", result[4], str)
		}
		matchFound = true
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to read log file")
	}

	if !matchFound {
		return errors.Wrap(err, "failed to parse ureadahead pack dump")
	}

	if sizeKB < minAcceptableUreadaheadPackSizeKB {
		return errors.Errorf("failed due to pack size %d kB too small. It is expected to be min %d kb", sizeKB, minAcceptableUreadaheadPackSizeKB)
	}

	return nil
}
