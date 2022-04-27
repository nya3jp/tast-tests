// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"bytes"
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/remote/bundles/cros/autoupdate/autoupdatelib"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

const (
	chromeDefaultUsername = "testuser@gmail.com"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NToMChromeLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify cross version vault's compatibility",
		Contacts: []string{
			"hcyang@google.com", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"tpm", "reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: autoupdatelib.TotalTestTime,
	})
}

func NToMChromeLogin(ctx context.Context, s *testing.State) {
	var err error
	env := &autoupdatelib.HwsecEnv{}
	env.CmdRunner = hwsecremote.NewCmdRunner(s.DUT())
	env.Helper, err = hwsecremote.NewHelper(env.CmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	env.Utility = env.Helper.CryptohomeClient()

	clearTpm := func(ctx context.Context, s *testing.State, env *autoupdatelib.HwsecEnv) {
		// Resets the TPM states before running the tests.
		if err := env.Helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
			s.Fatal("Failed to ensure resetting TPM: ", err)
		}
		if err := env.Helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
			s.Fatal("Failed to wait for TPM to be owned: ", err)
		}
	}

	// Connect to the chrome service server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := ui.NewChromeServiceClient(cl.Conn)

	ops := &autoupdatelib.Operations{
		PreUpdate: func(ctx context.Context, s *testing.State) {
			clearTpm(ctx, s, env)
		},
		PostUpdate: func(ctx context.Context, s *testing.State) {
			createUserAndCreateTestFile(ctx, s, env, cr)
		},
		PostRollback: func(ctx context.Context, s *testing.State) {
			loginUserAndReadTestFile(ctx, s, env, cr)
		},
	}

	autoupdatelib.NToMTest(ctx, s, ops, 3 /*deltaM*/)
}

func createUserAndCreateTestFile(ctx context.Context, s *testing.State, env *autoupdatelib.HwsecEnv, cr ui.ChromeServiceClient) {
	// Login Chrome for first time, and create a test file.
	if _, err := cr.New(ctx, &ui.NewRequest{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	sanitizedName, err := env.Utility.GetSanitizedUsername(ctx, chromeDefaultUsername, false)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}

	if err := hwsec.WriteUserTestContent(ctx, env.Utility, env.CmdRunner, sanitizedName, autoupdatelib.TestFile, autoupdatelib.TestFileContent); err != nil {
		s.Fatal("Failed to write test content: ", err)
	}

	if _, err := cr.Close(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}
}

func loginUserAndReadTestFile(ctx context.Context, s *testing.State, env *autoupdatelib.HwsecEnv, cr ui.ChromeServiceClient) {
	// Login Chrome to see if the test file still exists
	if _, err := cr.New(ctx, &ui.NewRequest{KeepState: true}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	sanitizedName, err := env.Utility.GetSanitizedUsername(ctx, chromeDefaultUsername, false)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}

	if content, err := hwsec.ReadUserTestContent(ctx, env.Utility, env.CmdRunner, sanitizedName, autoupdatelib.TestFile); err != nil {
		s.Fatal("Failed to read test content: ", err)
	} else if !bytes.Equal(content, []byte(autoupdatelib.TestFileContent)) {
		s.Fatalf("Unexpected test file content: got %q, want %q", string(content), autoupdatelib.TestFileContent)
	}

	if _, err := cr.Close(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}
}
