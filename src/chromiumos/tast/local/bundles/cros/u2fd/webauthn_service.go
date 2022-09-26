// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
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

	cr           *chrome.Chrome
	br           *browser.Browser
	closeBrowser uiauto.Action
	// Keeping keyboard in state instead of creating it each time because it takes about 5 seconds to create a keyboard.
	keyboard *input.KeyboardEventWriter
	conn     *chrome.Conn

	cfg      webauthnConfig
	password string
}

func (c *WebauthnService) New(ctx context.Context, req *hwsec.NewRequest) (*empty.Empty, error) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to restart ui job")
	}

	var bt browser.Type
	if req.GetBrowserType() == hwsec.BrowserType_ASH {
		bt = browser.TypeAsh
	} else {
		bt = browser.TypeLacros
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get keyboard")
	}

	var opts []chrome.Option
	if req.GetKeepState() {
		opts = append(opts, chrome.KeepState())
	}

	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		keyboard.Close()
		return nil, errors.Wrapf(err, "failed to log in by Chrome with %v browser", bt)
	}
	c.keyboard = keyboard
	c.cr = cr
	c.br = br
	c.closeBrowser = closeBrowser

	return &empty.Empty{}, nil
}

func (c *WebauthnService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.closeBrowser != nil {
		c.closeBrowser(ctx)
		c.br = nil
	}
	if c.cr != nil {
		c.cr.Close(ctx)
		c.cr = nil
	}
	if c.keyboard != nil {
		c.keyboard.Close()
		c.keyboard = nil
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartWebauthn(ctx context.Context, req *hwsec.StartWebauthnRequest) (*empty.Empty, error) {
	c.cfg = webauthnConfig{
		userVerification:  req.GetUserVerification(),
		authenticatorType: req.GetAuthenticatorType(),
		hasDialog:         req.GetHasDialog(),
	}
	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := c.br.NewConn(ctx, "https://webauthn.io/?"+getQueryStringByConfiguration(c.cfg))
	if err != nil {
		return nil, errors.Wrap(err, "failed to navigate to test website")
	}
	c.conn = conn

	return &empty.Empty{}, nil
}

func (c *WebauthnService) EndWebauthn(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.conn != nil {
		c.conn.Navigate(ctx, "https://webauthn.io/logout")
		c.conn.CloseTarget(ctx)
		c.conn.Close()
		c.conn = nil
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartMakeCredential(ctx context.Context, req *hwsec.StartMakeCredentialRequest) (*empty.Empty, error) {
	// Perform MakeCredential on the test website.

	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get test API connection")
	}

	testing.ContextLogf(ctx, "Username: %s", req.GetUsername())
	err = c.conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email')._x_model.set("%s")`, req.GetUsername()), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression to set username")
	}

	ui := uiauto.New(tconn)

	popupMessageNode := nodewith.ClassName("MessagePopupView")

	if !c.cfg.hasDialog {
		// If we will check the popup alert dialog later, wait for existing popup dialog
		// to disappear first.
		if err := ui.WaitUntilGone(popupMessageNode)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for power button press prompt gone")
		}
	}

	// Press "Register" button.
	err = c.conn.Eval(ctx, `document.getElementById('register-button').click()`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression to press register button")
	}

	// If authenticator type is "Platform", there's only platform option so we don't have to manually click "This device".
	if c.cfg.authenticatorType != hwsec.AuthenticatorType_PLATFORM {
		// Choose platform authenticator.
		platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
		if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to select platform authenticator from transport selection sheet")
		}
		if err := ui.DoDefault(platformAuthenticatorButton)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to click button for platform authenticator")
		}
	}

	if c.cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for the ChromeOS dialog")
		}
	} else {
		// Wait for popup alert dialog prompting for power button press.
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(popupMessageNode)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for power button press prompt")
		}
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) CheckMakeCredential(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := util.CheckMakeCredentialSuccessInWebAuthnIo(ctx, c.conn); err != nil {
		return nil, errors.Wrap(err, "failed to perform MakeCredential")
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) StartGetAssertion(ctx context.Context, req *hwsec.StartGetAssertionRequest) (*empty.Empty, error) {
	// Perform GetAssertion on the test website.

	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get test API connection")
	}

	err = c.conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email')._x_model.set("%s")`, req.GetUsername()), nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression to set username")
	}

	ui := uiauto.New(tconn)

	popupMessageNode := nodewith.ClassName("MessagePopupView")

	if !c.cfg.hasDialog {
		// If we will check the popup alert dialog later, wait for existing popup dialog
		// to disappear first.
		if err := ui.WaitUntilGone(popupMessageNode)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for power button press prompt gone")
		}
	}

	// Press "Login" button.
	err = c.conn.Eval(ctx, `document.getElementById('login-button').click()`, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute JS expression to press login button")
	}

	if c.cfg.hasDialog {
		// Wait for ChromeOS WebAuthn dialog.
		dialog := nodewith.ClassName("AuthDialogWidget")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for the ChromeOS dialog")
		}
	} else {
		// Wait for popup alert dialog prompting for power button press.
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(popupMessageNode)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to wait for power button press prompt")
		}
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) CheckGetAssertion(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := util.CheckGetAssertionSuccessInWebAuthnIo(ctx, c.conn); err != nil {
		return nil, errors.Wrap(err, "failed to perform GetAssertion")
	}
	return &empty.Empty{}, nil
}

func (c *WebauthnService) EnterPassword(ctx context.Context, req *hwsec.EnterPasswordRequest) (*empty.Empty, error) {
	if err := c.keyboard.Type(ctx, req.GetPassword()+"\n"); err != nil {
		return nil, errors.Wrap(err, "failed to type password into ChromeOS auth dialog")
	}
	return &empty.Empty{}, nil
}

func authenticatorTypeToValue(t hwsec.AuthenticatorType) string {
	switch t {
	case hwsec.AuthenticatorType_UNSPECIFIED:
		return ""
	case hwsec.AuthenticatorType_CROSS_PLATFORM:
		return "cross_platform"
	case hwsec.AuthenticatorType_PLATFORM:
		return "platform"
	}
	return "unknown"
}

func getQueryStringByConfiguration(cfg webauthnConfig) string {
	requireUv := cfg.userVerification == hwsec.UserVerification_REQUIRED
	return fmt.Sprintf(
		"regRequireUserVerification=%t"+
			"&attestation=none"+
			"&attachment=%s"+
			"&algES256=true&algRS256=true"+
			"&authRequireUserVerification=%t",
		requireUv, authenticatorTypeToValue(cfg.authenticatorType), requireUv)
}
