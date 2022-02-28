// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DumpVPDLog,
		Desc:     "Verify the behaviour of dump_vpd_log",
		Contacts: []string{"vsavu@chromium.org", "informational"},
		Attr:     []string{"group:mainline", "informational"},
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
	cmd := testexec.CommandContext(ctx, "/usr/sbin/dump_vpd_log")
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
