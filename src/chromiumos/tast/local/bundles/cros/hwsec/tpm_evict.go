// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TpmEvict,
		Desc: "Tests the TPM under low-resource conditions",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 8 * time.Minute,
	})
}

const (
	// Assign this path will let chaps use memory backed storage.
	scratchpadPath = "/tmp/chaps/"
	testIterations = 30
)

// TpmEvict verifies that PKCS #11 services remain functional when the TPM is
// operating under low-resource conditions. Specifically, more keys are used than
// are able to fit in TPM memory which requires that previously loaded keys be
// evicted. The test exercises the eviction code path as well as the reload code
// path (when a previously evicted key is used again).
func TpmEvict(ctx context.Context, s *testing.State) {
	r := libhwseclocal.NewCmdRunner()

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

	slot, err := pkcs11test.LoadP11TestToken(ctx, r, scratchpadPath, "1234")
	if err != nil {
		s.Error("Failed to load token using chaps_client: ", err)
	}

	// Inject large amount of key pairs to force previously loaded keys be evicted.
	for i := 0; i < testIterations; i++ {
		if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--label="+strconv.Itoa(i), "--replay_wifi"); err != nil {
			s.Errorf("p11_replay inject and replay failed on iteration %d: %v", i, err)
		}
	}
	for i := 0; i < testIterations; i++ {
		if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject", "--label="+strconv.Itoa(i+testIterations)); err != nil {
			s.Errorf("p11_replay inject failed on iteration %d: %v", i, err)
		}
	}
	for i := 0; i < testIterations; i++ {
		if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--label="+strconv.Itoa(i), "--replay_wifi", "--skip_generate"); err != nil {
			s.Errorf("p11_replay replay failed on iteration %d: %v", i, err)
		}
	}
}
