// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	defaultUsername = "testuser@gmail.com"
	defaultPassword = "testpass"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeServiceGRPC,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check basic functionality of ChromeService",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
		Params: []testing.Param{{
			Name: "default_fake_login",
			Val:  &pb.NewRequest{},
		}, {
			Name: "fake_login",
			Val: &pb.NewRequest{
				LoginMode: pb.LoginMode_LOGIN_MODE_FAKE_LOGIN,
				Credentials: &pb.NewRequest_Credentials{
					Username: defaultUsername,
					Password: defaultPassword,
				},
				TryReuseSession: false,
				KeepState:       false,
				EnableFeatures:  []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
				ExtraArgs:       []string{"--enable-logging"},
			},
		}, {
			Name: "fake_login_try_reuse_sessions",
			Val: &pb.NewRequest{
				LoginMode: pb.LoginMode_LOGIN_MODE_FAKE_LOGIN,
				Credentials: &pb.NewRequest_Credentials{
					Username: defaultUsername,
					Password: defaultPassword,
				},
				TryReuseSession: true,
				KeepState:       true,
				// Requesting the same features and args in order to reuse the same session
				EnableFeatures: []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
				ExtraArgs:      []string{"--enable-logging"},
			},
		}, {
			Name: "gaia_login",
			Val: &pb.NewRequest{
				// The test definition block has no access to testing.State and "ui.gaiaPoolDefault".
				// Credentials will be populated based on "ui.gaiaPoolDefault" in the main test function.
				LoginMode: pb.LoginMode_LOGIN_MODE_GAIA_LOGIN,
			},
		}, {
			Name: "default_fake_login_lacros",
			Val: &pb.NewRequest{
				// Default to using Rootfs Lacros.
				Lacros: &pb.Lacros{Selection: pb.Lacros_SELECTION_ROOTFS}},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "disabled_lacros",
			Val: &pb.NewRequest{
				Lacros: &pb.Lacros{Mode: pb.Lacros_MODE_DISABLED}},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// ChromeServiceGRPC tests ChromeService functionalities for managing chrome lifecycle.
func ChromeServiceGRPC(ctx context.Context, s *testing.State) {
	cl, err := crosserverutil.GetGRPCClient(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Populate credentials from Tast variable for the Gaia login test case.
	loginReq := s.Param().(*pb.NewRequest)
	if loginReq.LoginMode == pb.LoginMode_LOGIN_MODE_GAIA_LOGIN && loginReq.Credentials == nil {
		if loginReq.Credentials, err = pickRandomCreds(s.RequiredVar("ui.gaiaPoolDefault")); err != nil {
			s.Fatal("Failed to get login credentials: ", err)
		}
	}

	// Start Chrome on DUT
	cs := pb.NewChromeServiceClient(cl.Conn)
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Close Chrome on DUT
	if _, err := cs.Close(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// pickRandomCreds picks a random user and password from a list of credentials.
//
// creds is a string containing multiple credentials separated by newlines:
//
//	user1:pass1
//	user2:pass2
//	user3:pass3
//	..
func pickRandomCreds(creds string) (*pb.NewRequest_Credentials, error) {
	// Pick a random line
	lines := strings.Split(creds, "\n")
	randomIndex := random.Intn(len(lines))
	line := lines[randomIndex]

	// Extract user and password from the concatenated string
	line = strings.TrimSpace(line)
	userNamePassword := strings.SplitN(line, ":", 2)
	if len(userNamePassword) != 2 {
		return nil, errors.Errorf("failed to parse credential list: line %d: does not contain a colon", randomIndex+1)
	}
	return &pb.NewRequest_Credentials{
		Username: userNamePassword[0],
		Password: userNamePassword[1],
	}, nil
}
