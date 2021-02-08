// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         RebootTest,
		Desc:         "Verifies that the TPM ownership can be cleared",
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"group:hwsec_destructive_func"},
		ServiceDeps:  []string{"tast.cros.hwsec.AttestationClientService"},
	})
}

func RebootTest(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}

	helper, err := hwsecremote.NewHelperWithAttestationClient(r, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
}
