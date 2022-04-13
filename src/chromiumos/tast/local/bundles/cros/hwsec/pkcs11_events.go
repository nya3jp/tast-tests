// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Pkcs11Events,
		Desc: "Tests the response of the PKCS #11 system to login events",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 4 * time.Minute,
	})
}

// Pkcs11Events test the response of the PKCS #11 system to load /unload events.
func Pkcs11Events(ctx context.Context, s *testing.State) {
	r := libhwseclocal.NewCmdRunner()

	const numOfTokens = 2
	const numofEvents = 20

	var tokenList [numOfTokens]string
	// Setup token directories for testing.
	for i := 0; i < numOfTokens; i++ {
		// Assign this path will let chaps use memory backed storage.
		tokenList[i] = fmt.Sprintf("/tmp/chaps%d", i)
		if err := pkcs11test.SetupP11TestToken(ctx, r, tokenList[i]); err != nil {
			s.Fatal("Failed to initialize the scratchpad space: ", err)
		}
	}

	cleanupCtx := ctx
	// Give cleanup function 20 seconds to remove scratchpad.
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		for i := 0; i < numOfTokens; i++ {
			if err := pkcs11test.CleanupP11TestToken(ctx, r, tokenList[i]); err != nil {
				s.Fatal("Cleanup failed: ", err)
			}
		}
	}(cleanupCtx)

	// Setup a key on each token.
	for i := 0; i < numOfTokens; i++ {
		slot, err := pkcs11test.LoadP11TestToken(ctx, r, tokenList[i], tokenList[i])
		if err != nil {
			s.Error("Failed to load token using chaps_client (1): ", err)
		}
		if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--inject"); err != nil {
			s.Error("Load token failed (1): ", err)
		}
		if err := pkcs11test.UnloadP11TestToken(ctx, r, tokenList[i]); err != nil {
			s.Error("Failed to unload token using chaps_client (1): ", err)
		}
	}
	// Test a load by an immediate unload.
	for i := 0; i < numOfTokens; i++ {
		_, err := pkcs11test.LoadP11TestToken(ctx, r, tokenList[i], tokenList[i])
		if err != nil {
			s.Error("Failed to load token using chaps_client (2): ", err)
		}
		if err := pkcs11test.UnloadP11TestToken(ctx, r, tokenList[i]); err != nil {
			s.Error("Failed to unload token using chaps_client (2): ", err)
		}
	}

	// Test the tokens with a random load / unload events.
	seed := time.Now().UnixNano()
	s.Logf("Random seed: %d", seed)
	rnd := rand.New(rand.NewSource(seed))
	for i := 0; i < numOfTokens; i++ {
		token := rnd.Intn(2)
		// 0 is load, 1 is unload.
		event := rnd.Intn(2)

		if event == 0 {
			slot, err := pkcs11test.LoadP11TestToken(ctx, r, tokenList[token], tokenList[token])
			if err != nil {
				s.Error("Failed to load token using chaps_client (3): ", err)
			}
			if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
				s.Error("Load token failed (2): ", err)
			}
		} else {
			// The execution flow is random, so it's possible to hit error to unload a token that doesn't exist.
			pkcs11test.UnloadP11TestToken(ctx, r, tokenList[token])
		}
	}

	// Unload all tokens, it's possible to hit error to unload a token that doesn't exist.
	for i := 0; i < numOfTokens; i++ {
		pkcs11test.UnloadP11TestToken(ctx, r, tokenList[i])
	}
	// See if each token is still functional.
	for i := 0; i < numOfTokens; i++ {
		slot, err := pkcs11test.LoadP11TestToken(ctx, r, tokenList[i], tokenList[i])
		if err != nil {
			s.Error("Failed to load token using chaps_client (4): ", err)
		}
		if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--replay_wifi"); err != nil {
			s.Error("Load token failed (3): ", err)
		}
		if err := pkcs11test.UnloadP11TestToken(ctx, r, tokenList[i]); err != nil {
			s.Error("Failed to unload token using chaps_client (4): ", err)
		}
	}
}
