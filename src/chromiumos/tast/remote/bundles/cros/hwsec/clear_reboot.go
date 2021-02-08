// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClearReboot,
		Desc:         "Clear and Reboot",
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"group:hwsec_destructive_func"},
	})
}

func ClearReboot(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	out, err := r.Run(ctx, "base64", "/mnt/stateful_partition/encrypted.key")
	if err != nil {
		s.Fatal("Failed to get base64: ", err)
	}
	s.Log(string(out))

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ready TPM: ", err)
	}

	s.Log("Reboot")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	s.Log("Reboot fin")

	out, err = r.Run(ctx, "base64", "/mnt/stateful_partition/encrypted.key")
	if err != nil {
		s.Fatal("Failed to get base64: ", err)
	}
	s.Log(string(out))
}
