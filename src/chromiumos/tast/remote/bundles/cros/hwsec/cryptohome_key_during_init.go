// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	libhwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CryptohomeKeyDuringInit,
		Desc:         "Verifies that the TPM ownership can be cleared and taken",
		Contacts:     []string{"cylai@google.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Attr:         []string{"informational"},
	})
}

func CryptohomeKeyDuringInit(ctx context.Context, s *testing.State) {
	r, err := libhwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := libhwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	username := "unowned@gmail.com"
	passwd := "testpass"

	mountTask := hwsec.NewStressTaskMount(utility, username, passwd)
	ownershipTask := hwsec.NewStressTaskTakeOwnership(&helper.Helper)
	hwsec.RegisterRunner(mountTask, "mount")
	hwsec.RegisterRunner(ownershipTask, "ownership")
	topJSON := `{
		"primary": {
			"names": ["ownership"],
			"count": 1,
			"cof":false
		},
		"secondary": [
			{
				"names": ["mount"],
				"cof":true
			}
		]
	}`
	const maxRun int = 5
	for i := 0; i < maxRun; i++ {
		s.Log("fuck")
		tester, err := hwsec.UnmarshalPSTaskModel(topJSON)
		if err != nil {
			s.Fatal("Failed to unmarshal primary-secondary task model: ", err)
		}
		tester.Run(ctx, nil)
		if err := helper.EnsureTPMIsReset(ctx); err != nil {
			s.Fatal("Failed to ensure resetting TPM: ", err)
		}
	}
	checkTask := hwsec.NewStressTaskCheckKey(utility, username, passwd)
	hwsec.RegisterRunner(checkTask, "check")
	topJSON = `{
		"primary": {
			"names": ["ownership"],
			"count": 1,
			"cof":false
		},
		"secondary": [
			{
				"names": ["check"],
				"cof":true
			}
		]
	}`
	for i := 0; i < maxRun; i++ {
		s.Log("fuck")
		tester, err := hwsec.UnmarshalPSTaskModel(topJSON)
		if err != nil {
			s.Fatal("Failed to unmarshal primary-secondary task model: ", err)
		}
		tester.Run(ctx, nil)
		if err := helper.EnsureTPMIsReset(ctx); err != nil {
			s.Fatal("Failed to ensure resetting TPM: ", err)
		}
	}
}
