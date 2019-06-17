// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/testing"
)

const (
	defaultTakingOwnershipTimeout int = 40 * 1000
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RetakeOwnership,
		Desc:         "Verifies that the TPM ownership can be cleared and taken",
		Contacts:     []string{"cylai@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"informational"},
	})
}

func RetakeOwnership(ctx context.Context, s *testing.State) {
	s.Log("Start test with creating a proxy")
	utility, err := libhwsec.NewUtility(ctx, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := libhwsec.EnsureTpmIsReset(ctx, utility); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")
	s.Log("Start taking ownership")
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, defaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Onwership is taken")
}
