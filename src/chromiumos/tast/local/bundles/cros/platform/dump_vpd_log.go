// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DumpVPDLog,
		Desc:         "Verify the behaviour of dump_vpd_log",
		Contacts:     []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vpd"},
	})
}

const cachePath = "/var/cache/vpd/full-v2.txt"

var vpdEntry = regexp.MustCompile(`^".+"=".*"$`)

func DumpVPDLog(ctx context.Context, s *testing.State) {
	// Force a restore of the VPD cache for later tests.
	defer func(ctx context.Context) {
		if err := os.Remove(cachePath); err != nil {
			s.Error("Failed to remove the VPD cache: ", err)
		}

		if err := runDumpVPDLog(ctx); err != nil {
			s.Error("Failed to restore the VPD cache: ", err)
		}

		if err := validateCache(); err != nil {
			s.Error("Restored cache is not valid: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Make sure cache is available.
	if err := runDumpVPDLog(ctx); err != nil {
		s.Fatal("Failed to create initial VPD cache: ", err)
	}

	if err := validateCache(); err != nil {
		s.Fatal("Initial cache is not valid: ", err)
	}

	// Regenerate missing cache.
	if err := os.Remove(cachePath); err != nil {
		s.Error("Failed to remove the VPD cache: ", err)
	}

	if err := runDumpVPDLog(ctx); err != nil {
		s.Fatal("Failed to regenerate VPD cache: ", err)
	}

	if err := validateCache(); err != nil {
		s.Fatal("Regenerated cache is not valid: ", err)
	}

	// Regenerate invalid cache.
	file, err := os.OpenFile(cachePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal("Failed to open cache file: ", err)
	}
	defer file.Close()

	// Append a VPD read error.
	if _, err := file.Write([]byte("# RW_VPD execute error.\n")); err != nil {
		s.Fatal("Failed to append to cache file: ", err)
	}

	if err := file.Close(); err != nil {
		s.Fatal("Failed to close cache: ", err)
	}

	if err := runDumpVPDLog(ctx); err != nil {
		s.Fatal("Failed to regenerate broken VPD cache: ", err)
	}

	if err := validateCache(); err != nil {
		s.Fatal("Cache not regenerated: ", err)
	}
}

func runDumpVPDLog(ctx context.Context) error {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to create output directory")
	}

	now := time.Now()
	basename := fmt.Sprintf("dump_vpd_log.%s", now)

	outfile, err := os.Create(filepath.Join(outdir, basename+".out"))
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer outfile.Close()

	errfile, err := os.Create(filepath.Join(outdir, basename+".err"))
	if err != nil {
		return errors.Wrap(err, "failed to create error file")
	}
	defer errfile.Close()

	cmd := testexec.CommandContext(ctx, "/usr/sbin/dump_vpd_log")

	cmd.Stdout = outfile
	cmd.Stderr = errfile

	testing.ContextLogf(ctx, "Running dump_vpd_log, dumping output as %q", basename)
	return cmd.Run(testexec.DumpLogOnError)
}

func validateCache() error {
	if _, err := os.Stat(cachePath); err != nil {
		return errors.Wrapf(err, "failed to find cache %q", cachePath)
	}

	file, err := os.Open(cachePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open cache %q", cachePath)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if !vpdEntry.MatchString(scanner.Text()) {
			return errors.Errorf("line %q in cache is not valid", scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "failed to read cache %q", cachePath)
	}

	return nil
}
