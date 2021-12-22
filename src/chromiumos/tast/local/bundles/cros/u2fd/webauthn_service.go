// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			hwsec.RegisterWebauthnServiceServer(srv, &WebauthnService{s: s})
		},
	})
}

type webauthnConfig struct {
	userVerification  hwsec.UserVerification
	authenticatorType hwsec.AuthenticatorType
	hasDialog         bool
}

// WebauthnService implements tast.cros.hwsec.WebauthnService.
type WebauthnService struct {
	s *testing.ServiceState

	cr        *chrome.Chrome
	fdms      *fakedms.FakeDMS
	logReader *syslog.ChromeReader
	tconn     *chrome.TestConn
	keyboard  *input.KeyboardEventWriter
	conn      *chrome.Conn

	tmpDir   string
	cfg      webauthnConfig
	password string
}

func (c *WebauthnService) New(ctx context.Context, req *hwsec.NewRequest) (*empty.Empty, error) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to restart ui job")
	}

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tempdir")
	}
	c.tmpDir = tmpDir

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: req.GetUsername(), Pass: req.GetPassword()}),
		// Enable device event log in Chrome logs for validation.
		chrome.ExtraArgs("--vmodule=device_event_log*=1"),
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to log in by Chrome")
	}
	c.cr = cr
	c.password = req.GetPassword()

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Chrome log reader")
	}
	c.logReader = logReader

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting test API connection failed")
	}
	c.tconn = tconn

	return &empty.Empty{}, nil
}

func (c *WebauthnService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		c.cr.Close(ctx)
		c.cr = nil
	}
	if c.fdms != nil {
		c.fdms.Stop(ctx)
		c.fdms = nil
	}
	if c.logReader != nil {
		c.logReader.Close()
		c.logReader = nil
	}
	if c.keyboard != nil {
		c.keyboard.Close()
		c.keyboard = nil
	}
	if c.tmpDir != "" {
		os.RemoveAll(c.tmpDir)
		c.tmpDir = ""
	}
	c.tconn = nil
	return &empty.Empty{}, nil
}

func (c *WebauthnService) SetupPIN(ctx context.Context, req *hwsec.SetupPINRequest) (*empty.Empty, error) {
	_, err := util.SetUpUserPIN(ctx, c.cr, req.GetPin(), c.password, false /*autosubmit*/)
	if err != nil {
		return nil, err
	}
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get keyboard")
	}
	c.keyboard = keyboard

	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartWebauthn(ctx context.Context, req *hwsec.StartWebauthnRequest) (*empty.Empty, error) {
	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := c.cr.NewConn(ctx, "https://webauthn.io/")
	if err != nil {
		return nil, errors.Wrap(err, "failed to navigate to test website")
	}
	c.conn = conn
	c.cfg = webauthnConfig{
		userVerification:  req.GetUserVerification(),
		authenticatorType: req.GetAuthenticatorType(),
		hasDialog:         req.GetHasDialog(),
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) EndWebauthn(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartMakeCredential(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// Perform MakeCredential on the test website.

	// Use a random username because webauthn.io keeps state for each username for a period of time.
	err := c.conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email').value = "%s"`, randomUsername()), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression")
	}

	// Select "Authenticator Type"
	err = c.conn.Eval(ctx, fmt.Sprintf(`document.getElementById('select-authenticator').value= "%s"`, authenticatorTypeToValue(c.cfg.authenticatorType)), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression")
	}

	// Select "User Verification"
	err = c.conn.Eval(ctx, fmt.Sprintf(`document.getElementById('select-verification').value= "%s"`, userVerificationToValue(c.cfg.userVerification)), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression")
	}

	// Press "Register" button
	err = c.conn.Eval(ctx, `document.getElementById('register-button').click()`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression")
	}

	ui := uiauto.New(c.tconn)

	// If authenticator type is "Platform", there's only platform option so we don't have to manually click "This device".
	if c.cfg.authenticatorType != hwsec.AuthenticatorType_PLATFORM {
		// Choose platform authenticator
		platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
		if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to select platform authenticator from transport selection sheet")
		}
		if err := ui.LeftClick(platformAuthenticatorButton)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to click button for platform authenticator")
		}
	}

	if c.cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return nil, errors.Wrap(err, "ChromeOS dialog did not show up")
		}
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) CheckMakeCredentialSuccess(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := util.AssertMakeCredentialSuccess(ctx, c.logReader); err != nil {
		return nil, errors.Wrap(err, "MakeCredential did not succeed")
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartGetAssertion(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// Perform GetAssertion on the test website.

	ui := uiauto.New(c.tconn)

	// Press "Login" button.
	err := c.conn.Eval(ctx, `document.getElementById('login-button').click()`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression")
	}

	if c.cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return nil, errors.Wrap(err, "ChromeOS dialog did not show up")
		}
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) CheckGetAssertionSuccess(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := util.AssertGetAssertionSuccess(ctx, c.logReader); err != nil {
		return nil, errors.Wrap(err, "GetAssertion did not succeed")
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) EnterPin(ctx context.Context, req *hwsec.EnterPinRequest) (*empty.Empty, error) {
	if err := c.keyboard.Type(ctx, req.GetPin()+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
	}
	return &empty.Empty{}, nil
}

func userVerificationToValue(uv hwsec.UserVerification) string {
	switch uv {
	case hwsec.UserVerification_DISCOURAGED:
		return "discouraged"
	case hwsec.UserVerification_PREFERRED:
		return "preferred"
	case hwsec.UserVerification_REQUIRED:
		return "required"
	}
	return "unknown"
}

func authenticatorTypeToValue(t hwsec.AuthenticatorType) string {
	switch t {
	case hwsec.AuthenticatorType_UNSPECIFIED:
		return ""
	case hwsec.AuthenticatorType_CROSS_PLATFORM:
		return "cross-platform"
	case hwsec.AuthenticatorType_PLATFORM:
		return "platform"
	}
	return "unknown"
}

// randomUsername returns a random username of length 10.
func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 10)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}
