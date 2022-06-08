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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UreadaheadValidation,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
			// VM ureadahead should only be validated on 8GB+ boards. Please see arc.DataCollector for details.
			ExtraHardwareDeps: hwdep.D(hwdep.MinMemory(7500)),
		}},
		// Minimum acceptable.
		Timeout: 5 * time.Minute,
	})
}

func UreadaheadValidation(ctx context.Context, s *testing.State) {
	const (
		// Names of ureadahead dump logs.
		ureadaheadLogName      = "ureadahead.log"
		ureadaheadGuestLogName = "guest_ureadahead.log"

		// Normally generated host ureadahead pack covers >300MB of data.
		minAcceptableUreadaheadPackSizeKB = 300 * 1024
		// Guest ureadahead pack is smaller than host from stainless data.
		minAcceptableGuestUreadaheadPackSizeKB = 100 * 1024
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
		s.Fatalf("Expected ureadahead pack %s does not exist", packPath)
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
	if err = checkPackFileDump(ctx, logPath, minAcceptableUreadaheadPackSizeKB); err != nil {
		s.Fatalf("Failed to verify ureadahead pack file dump, please check %q: %v", ureadaheadLogName, err)
	}

	// If VM, also verify guest OS ureadahead dump.
	if vmEnabled {
		vmLogPath := filepath.Join(s.OutDir(), ureadaheadGuestLogName)
		if err = dumpGuestPack(ctx, vmLogPath); err != nil {
			s.Fatal("Failed to dump guest ureadahead pack: ", err)
		}

		// Verify the guest pack file dump.
		if err = checkPackFileDump(ctx, vmLogPath, minAcceptableGuestUreadaheadPackSizeKB); err != nil {
			s.Fatalf("Failed to verify guest ureadahead pack file dump, please check %q: %v", ureadaheadGuestLogName, err)
		}
	}
}

// dumpGuestPack dumps the ureadahead pack in the guest OS.
func dumpGuestPack(ctx context.Context, logPath string) error {
	// File path for ureadahead pack in the quest OS.
	const ureadaheadDataDir = "/var/lib/ureadahead"

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.UnRestrictARCCPU())
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

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
func checkPackFileDump(ctx context.Context, logPath string, minPackSize int) error {
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

	testing.ContextLogf(ctx, "Found ureadahead pack at %s with size of %d kB", logPath, sizeKB)
	if sizeKB < minPackSize {
		return errors.Errorf("failed due to pack size %d kB too small. It is expected to be min %d kB", sizeKB, minPackSize)
	}

	return nil
}
