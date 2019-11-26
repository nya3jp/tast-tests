// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11test

import (
	"context"

	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/errors"
)

// SignAndVerify is just a convenient runner to test both signing and verification.
// altInput is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func SignAndVerify(ctx context.Context, p *pkcs11.Util, key *pkcs11.KeyInfo, input string, altInput string, mechanism *pkcs11.MechanismInfo) error {
	// Test signing.
	if err := key.Sign(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of signed message.
	if err := key.Verify(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of another message (should fail).
	if err := key.Verify(ctx, p, altInput, input+".sig", mechanism); err == nil {
		// Should not happen.
		return errors.Errorf("verification functionality for %s failed, corrupted message is verified", mechanism.Name)
	}
	return nil
}
