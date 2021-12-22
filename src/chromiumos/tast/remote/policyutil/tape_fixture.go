// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	//"chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	ts "chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            fixture.Tape,
		Desc:            "Fixture providing enrollment",
		Contacts:        []string{"alexanderhartl@google.com", "cros-engprod-muct@google.com"},
		Impl:            &tapeFixt{},
		SetUpTimeout:    10 * time.Minute,
		TearDownTimeout: 2 * time.Minute,
		ResetTimeout:    15 * time.Second,
		ServiceDeps:     []string{"tast.cros.tape.TapeService"},
	})
}

type tapeFixt struct {
	tokenDir  string
	tokenFile string
	token     string
	done      int
}

func (t *tapeFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	//t.tokenDir = tape.TokenDir
	//t.tokenFile = tape.TokenFile
	t.tokenDir = "/tmp"
	t.tokenFile = "token.json"

	done := make(chan bool)
	tapeFixt.done = done

	testing.ContextLog(ctx, "Start loop")
	go t.tokenGenerationLoop(ctx, s)
	testing.Sleep(ctx, 20*time.Second)

	return nil
}

func (t *tapeFixt) tokenGeneration(ctx context.Context, s *testing.FixtState, tc ts.TapeServiceClient) {

	//	token, err := tape.TokenString(ctx)
	//	if err != nil {
	//		testing.ContextLog(ctx, "failed to get token")
	//		return
	//	}
	//	t.token = token
	t.token = "TEST"

	if _, err := tc.CreateTokenFile(ctx, &ts.CreateTokenFileRequest{
		Path:  t.tokenDir,
		File:  t.tokenFile,
		Token: t.token,
	}); err != nil {
		s.Fatal("Failed to create token on DUT: ", err)
		testing.ContextLog(ctx, "Failed to create token on DUT")
		return
	}
}

func (t *tapeFixt) tokenGenerationLoop(ctx context.Context, s *testing.FixtState) {

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		testing.ContextLog(ctx, "Failed to dail: ", err)
		<-tapeFixt.done
		return
	}
	defer cl.Close(ctx)

	tc := ts.NewTapeServiceClient(cl.Conn)

	for {
		select {
		case <-tapeFixt.done:
			return
		default:
			testing.ContextLog(ctx, "LOOP")
			if t.done == 1 {
				break
			}
			t.tokenGeneration(ctx, s, tc)
			testing.Sleep(ctx, time.Second*10)
		}
	}
}

func (t *tapeFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	tapeFixt.done <- true

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	tc := ts.NewTapeServiceClient(cl.Conn)

	if _, err := tc.RemoveTokenFile(ctx, &ts.RemoveTokenFileRequest{
		Path: t.tokenDir,
	}); err != nil {
		s.Fatal("Failed to remove temporary directory for token: ", err)
	}
}

func (*tapeFixt) Reset(ctx context.Context) error                        { return nil }
func (*tapeFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*tapeFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
