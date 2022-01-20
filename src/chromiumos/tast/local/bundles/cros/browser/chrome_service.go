// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/browser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterChromeServiceServer(srv,
				&ChromeService{s: s, sharedObject: common.SharedObjectsForServiceSingleton})
		},
		GuaranteeCompatibility: true,
	})
}

// ChromeService implements tast.cros.browser.ChromeService
type ChromeService struct {
	s            *testing.ServiceState
	sharedObject *common.SharedObjectsForService
	mutex        sync.Mutex
}

// defaultCreds is the default credentials used for fake logins.
var defaultCreds = chrome.Creds{
	User: "testuser@gmail.com",
	Pass: "testpass",
}

// New logs into Chrome with the supplied chrome options.
func (svc *ChromeService) New(ctx context.Context, req *pb.NewRequest) (*empty.Empty, error) {
	//Follow the same locking sequence from svc lock to shared object lock to avoid deadlock
	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	opts, err := toOptions(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to chrome options")
	}

	// By default, this will always create a new chrome session even when there is an existing one.
	// This gives full control of the lifecycle to the end users.
	// Users can use TryReuseSessions if they want to potentially reuse the session.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}

	// Store the newly created chrome sessions in the shared object so other services can use it.
	svc.sharedObject.Chrome = cr

	return &empty.Empty{}, nil
}

// Close closes all surfaces and Chrome.
// This will likely be called in a defer in remote tests instead of called explicitly.
func (svc *ChromeService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if svc.sharedObject.Chrome == nil {
		return nil, errors.New("Chrome not available")
	}

	err := svc.sharedObject.Chrome.Close(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome: ", err)
	}

	svc.sharedObject.Chrome = nil
	return &empty.Empty{}, err
}

func toOptions(req *pb.NewRequest) ([]chrome.Option, error) {
	//TODO(jonfan): Find a creative way to unit test this function
	//The underlying object Config and MutableConfig are private
	//chrome.Option are callback functions that work on Config, and they cannot
	//be compared easily without having access to Config or its Mock Interface.
	var options []chrome.Option

	if req.KeepState {
		options = append(options, chrome.KeepState())
	}

	if req.TryReuseSession {
		options = append(options, chrome.TryReuseSession())
	}

	switch req.GetLoginMode() {
	case pb.LoginMode_NO_LOGIN:
		options = append(options, chrome.NoLogin())
	case pb.LoginMode_FAKE_LOGIN:
		options = append(options, chrome.FakeLogin(toCreds(req.Credentials)))
	case pb.LoginMode_GAIA_LOGIN:
		options = append(options, chrome.GAIALogin(toCreds(req.Credentials)))
	case pb.LoginMode_GUEST_LOGIN:
		options = append(options, chrome.GuestLogin())
	default:
		options = append(options, chrome.FakeLogin(defaultCreds))
	}

	if len(req.ExtraArgs) > 0 {
		options = append(options, chrome.ExtraArgs(req.ExtraArgs...))
	}

	if len(req.EnableFeatures) > 0 {
		options = append(options, chrome.EnableFeatures(req.EnableFeatures...))
	}

	if len(req.DisableFeatures) > 0 {
		options = append(options, chrome.DisableFeatures(req.DisableFeatures...))
	}

	return options, nil
}

func toCreds(c *pb.NewRequest_Credentials) chrome.Creds {
	return chrome.Creds{
		User:       c.Username,
		Pass:       c.Password,
		GAIAID:     c.GaiaId,
		Contact:    c.Contact,
		ParentUser: c.ParentUsername,
		ParentPass: c.ParentPassword,
	}
}
