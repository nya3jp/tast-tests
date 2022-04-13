// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Pkcs11LoadPerf,
		Desc: "Test pkcs11 load performance",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 4 * time.Minute,
	})
}

// Pkcs11LoadPerf test the chapsd load key performance.
func Pkcs11LoadPerf(ctx context.Context, s *testing.State) {
	r := libhwseclocal.NewCmdRunner()

	// Assign this path will let chaps use memory backed storage.
	scratchpadPath := "/tmp/chaps/"
	if err := pkcs11test.SetupP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}

	cleanupCtx := ctx
	// Give cleanup function 20 seconds to remove scratchpad.
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := pkcs11test.CleanupP11TestToken(ctx, r, scratchpadPath); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	// Check token load successfully.
	slot, err := pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth1")
	if err != nil {
		s.Error("Failed to load token using chaps_client (1): ", err)
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject"); err != nil {
		s.Error("Load token failed (1): ", err)
	}
	if err := pkcs11test.UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		s.Error("Failed to unload token using chaps_client (1): ", err)
	}
	slot, err = pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "auth1")
	if err != nil {
		s.Error("Failed to load token using chaps_client (1): ", err)
	}

	// List the objects and get timing data.
	// The output will have multiple lines like 'Elapsed: 25ms'. We are
	// interested in the first three values representing:
	// 1) How long it took to open a session.
	// 2) How long it took to list public objects.
	// 3) How long it took to list private objects.
	// The following code extracts the numeric value from each timing statement.
	count := 0
	elapsedTime := [3]int{0, 0, 0}
	lines, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--list_objects")
	if err != nil {
		s.Error("Failed to list objects: ", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(lines))
	re := regexp.MustCompile(`Elapsed: (\d+)ms`)
	for scanner.Scan() && count < 3 {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			if elapsedTime[count], err = strconv.Atoi(matches[1]); err != nil {
				s.Error("Convert string to integer failed: ", err)
			}
			count++
		}
	}
	if count != 3 {
		s.Error("Failed to get elapsed time")
	}

	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      "cert_read",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, float64(elapsedTime[0]+elapsedTime[1]))
	value.Set(perf.Metric{
		Name:      "key_ready",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, float64(elapsedTime[0]+elapsedTime[1]+elapsedTime[2]))
	value.Save(s.OutDir())
}
