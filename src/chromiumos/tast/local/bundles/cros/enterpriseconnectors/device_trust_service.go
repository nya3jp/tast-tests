// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterpriseconnectors

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	pb "chromiumos/tast/services/cros/enterpriseconnectors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterDeviceTrustServiceServer(srv, &DeviceTrustService{})
		},
	})
}

// DeviceTrustService implements tast.cros.enterpriseconnectors.DeviceTrustService.
type DeviceTrustService struct {
	cr *chrome.Chrome
}

// Enroll the device with the provided account credentials.
func (service *DeviceTrustService) Enroll(ctx context.Context, req *pb.EnrollRequest) (_ *empty.Empty, retErr error) {
	if service.cr != nil {
		return nil, errors.New("DUT for running snapshot is already set up")
	}
	var opts []chrome.Option

	opts = append(opts, chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.User, Pass: req.Pass}))
	opts = append(opts, chrome.DMSPolicy("https://crosman-alpha.sandbox.google.com/devicemanagement/data/api"))
	opts = append(opts, chrome.NoLogin())
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	service.cr = cr
	return &empty.Empty{}, nil
}

// LoginWithFakeIDP uses the fake user credentials to get a SAML redirection to a Fake IDP, where the Device Trust attestation flow is tested.
func (service *DeviceTrustService) LoginWithFakeIDP(origCtx context.Context, req *pb.LoginWithFakeIDPRequest) (res *pb.LoginWithFakeIDPResponse, retErr error) {
	var fakeCreds chrome.Creds
	fakeCreds.User = "tast-test-device-trust@managedchrome.com"

	cr, err := chrome.New(
		origCtx,
		chrome.KeepEnrollment(),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.DontWaitForCryptohome(),
		chrome.DMSPolicy("https://crosman-alpha.sandbox.google.com/devicemanagement/data/api"),
		chrome.RemoveNotification(false),
		chrome.LoadSigninProfileExtension(req.SigninProfileTestExtensionManifestKey),
		chrome.SAMLLogin(fakeCreds),
		chrome.EnableFeatures("DeviceTrustConnectorEnabled"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "Chrome login failed")
	}

	tconn, err := cr.SigninProfileTestAPIConn(origCtx)
	if err != nil {
		return nil, errors.Wrap(err, "creating login test API connection failed")
	}

	loginPossible, err := testFakeIDP(origCtx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "Device Trust failed")
	}

	return &pb.LoginWithFakeIDPResponse{Succesful: loginPossible}, nil
}

const defaultUITimeout = 20 * time.Second

func activateDeviceTrustViaSettings(ctx context.Context, ui *uiauto.Context) error {
	root := nodewith.Name("IdP Settings").Role(role.RootWebArea)

	radioButtonYes := nodewith.Role(role.RadioButton).Ancestor(root).Nth(1)
	if err := uiauto.Combine("Click on yes",
		ui.WaitUntilExists(radioButtonYes),
		ui.LeftClick(radioButtonYes),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on yes in the IdP settings")
	}

	invalidateCheckbox := nodewith.Role(role.CheckBox).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Invalidate",
		ui.WaitUntilExists(invalidateCheckbox),
		ui.LeftClick(invalidateCheckbox),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Invalidate in the IdP settings")
	}

	saveButton := nodewith.Name("Save").Role(role.Button).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Save",
		ui.WaitUntilExists(saveButton),
		ui.LeftClick(saveButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Save in the IdP settings")
	}

	homeLink := nodewith.Name("Home").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on Home",
		ui.WaitUntilExists(homeLink),
		ui.LeftClick(homeLink),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Home in the IdP settings")
	}

	return nil
}

func loginPossible(ctx context.Context, ui *uiauto.Context) (bool, error) {
	root := nodewith.Name("Sample Login page").Role(role.RootWebArea)
	loginButton := nodewith.Name("Login").Role(role.Button).Ancestor(root).Focusable()
	unsuccesfulText := nodewith.Name("Please login from a trusted device.").Role(role.StaticText).Ancestor(root)

	result := false
	err := testing.Poll(ctx, func(ctx context.Context) error {
		err := ui.Exists(loginButton)(ctx)
		if err == nil {
			result = true
			return nil
		}
		err = ui.Exists(unsuccesfulText)(ctx)
		if err == nil {
			result = false
			return nil
		}
		return errors.Wrap(err, " found neither the login button nor the text \"Please login from a trusted device\"")
	}, &testing.PollOptions{Interval: 300 * time.Millisecond,
		Timeout: defaultUITimeout})

	if err != nil {
		return false, err
	}

	return result, nil
}

func testFakeIDP(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	ui := uiauto.New(tconn).WithTimeout(defaultUITimeout)
	root := nodewith.Name("Enterprise NTP").Role(role.RootWebArea)

	// Activate device trust requirement.
	settingsButton := nodewith.Name("Login Settings").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Go to settings",
		ui.WaitUntilExists(settingsButton),
		ui.LeftClick(settingsButton),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to go to settings")
	}
	if err := activateDeviceTrustViaSettings(ctx, ui); err != nil {
		return false, errors.Wrap(err, " failed to set settings")
	}

	testButton := nodewith.Name("App 1").Role(role.Link).Ancestor(root).Focusable()
	if err := uiauto.Combine("Click on OK and proceed",
		ui.WaitUntilExists(testButton),
		ui.LeftClick(testButton),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click OK. Is Account addition dialog open?")
	}

	loginPossible, err := loginPossible(ctx, ui)
	if err != nil {
		return false, errors.Wrap(err, " failed to load results")
	}

	return loginPossible, nil
}
