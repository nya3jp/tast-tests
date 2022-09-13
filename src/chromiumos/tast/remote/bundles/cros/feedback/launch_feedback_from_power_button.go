// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchFeedbackFromPowerButton,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "User is able to launch feedback app from power button",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		ServiceDeps: []string{
			"tast.cros.browser.ChromeService",
			"tast.cros.ui.AutomationService",
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"servo"},
		Timeout:      5 * time.Minute,
	})
}

// LaunchFeedbackFromPowerButton verifies launching feedback app from power button.
func LaunchFeedbackFromPowerButton(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	servoSpec := s.RequiredVar("servo")
	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}(cleanupCtx)

	loginChrome := func() (*rpc.Client, error) {
		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to RPC service on DUT")
		}

		// Start Chrome on the DUT.
		cs := ui.NewChromeServiceClient(cl.Conn)
		loginReq := &ui.NewRequest{EnableFeatures: []string{"OsFeedback"}}
		if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		return cl, nil
	}

	// Perform initial Chrome login.
	cl, err := loginChrome()
	if err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	uiautoSvc := ui.NewAutomationServiceClient(cl.Conn)

	// Press power button.
	if err := pxy.Servo().KeypressWithDuration(
		ctx, servo.PowerKey, servo.DurPress); err != nil {
		s.Fatal("Failed to power long press: ", err)
	}

	// Find feedback button and click.
	feedbackButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Name{Name: "Feedback"}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: feedbackButtonFinder}); err != nil {
		s.Fatal("Failed to find feedback button on DUT UI: ", err)
	}
	if _, err := uiautoSvc.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: feedbackButtonFinder}); err != nil {
		s.Fatal("Failed to click feedback button: ", err)
	}

	// Verify issue description input exists.
	issueDescriptionInputFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_TEXT_FIELD}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: issueDescriptionInputFinder}); err != nil {
		s.Fatal("Failed to find feedback issue description input: ", err)
	}

	// Verify continue button exists.
	continueButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Name{Name: "Continue"}},
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: continueButtonFinder}); err != nil {
		s.Fatal("Failed to find feedback continue button: ", err)
	}

	// Verify five default help content links exist.
	iframeFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_IFRAME}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}

	for i := 0; i < 5; i++ {
		helpLinkFinder := &ui.Finder{
			NodeWiths: []*ui.NodeWith{
				{Value: &ui.NodeWith_Ancestor{Ancestor: iframeFinder}},
				{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_LINK}},
				{Value: &ui.NodeWith_Nth{Nth: int32(i)}},
			},
		}
		if _, err := uiautoSvc.WaitUntilExists(
			ctx, &ui.WaitUntilExistsRequest{Finder: helpLinkFinder}); err != nil {
			s.Fatal("Failed to find feedback default help link: ", err)
		}
	}
}
