// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMLiveTests,
		Desc: "Runs cryptohome's TPM live tests, which test TPM keys, PCR, and NVRAM functionality",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"garryxiao@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}

func CryptohomeTPMLiveTests(ctx context.Context, s *testing.State) {
	if err := hwsec.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	// Waits for TPM to be owned.
	// TODO: replace this call with a hwsec util call when the util is ready before sending the CL for review.
	if err := ready.Wait(ctx, func(msg string) { s.Log(msg) }); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	if err := testexec.CommandContext(
		ctx, "cryptohome-tpm-live-test", "--tpm2_use_system_owner_password").Run(); err != nil {
		s.Fatal("TPM live test failed: ", err)
	}
}
