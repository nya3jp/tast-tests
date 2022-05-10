// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"bytes"
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/autoupdate/util"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
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
		Timeout: util.TotalTestTime,
	})
}

func NToMChromeLogin(ctx context.Context, s *testing.State) {
	env, err := util.NewHwsecEnv(s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec env: ", err)
	}

	ops := &util.Operations{
		PreUpdate: func(ctx context.Context) error {
			return util.ClearTpm(ctx, env)
		},
		PostUpdate: func(ctx context.Context) error {
			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)
			return createUserAndCreateTestFile(ctx, env, cl.Conn)
		},
		PostRollback: func(ctx context.Context) error {
			cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
			if err != nil {
				s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
			}
			defer cl.Close(ctx)
			return loginUserAndReadTestFile(ctx, env, cl.Conn)
		},
	}

	if err := util.NToMTest(ctx, s.DUT(), s.OutDir(), s.RPCHint(), ops, 3 /*deltaM*/); err != nil {
		s.Fatal("Failed to run cross version test: ", err)
	}
}

func createUserAndCreateTestFile(ctx context.Context, env *util.HwsecEnv, conn *grpc.ClientConn) error {
	cr := ui.NewChromeServiceClient(conn)

	// Login Chrome for first time, and create a test file.
	if _, err := cr.New(ctx, &ui.NewRequest{}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx, &empty.Empty{})

	if err := hwsec.WriteUserTestContent(ctx, env.Utility, env.CmdRunner, util.ChromeDefaultUsername, util.TestFile, util.TestFileContent); err != nil {
		return errors.Wrap(err, "failed to write test content")
	}

	return nil
}

func loginUserAndReadTestFile(ctx context.Context, env *util.HwsecEnv, conn *grpc.ClientConn) error {
	cr := ui.NewChromeServiceClient(conn)

	// Login Chrome to see if the test file still exists
	if _, err := cr.New(ctx, &ui.NewRequest{KeepState: true}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx, &empty.Empty{})

	if content, err := hwsec.ReadUserTestContent(ctx, env.Utility, env.CmdRunner, util.ChromeDefaultUsername, util.TestFile); err != nil {
		return errors.Wrap(err, "failed to read test content")
	} else if !bytes.Equal(content, []byte(util.TestFileContent)) {
		return errors.Errorf("unexpected test file content: got %q, want %q", string(content), util.TestFileContent)
	}

	return nil
}
