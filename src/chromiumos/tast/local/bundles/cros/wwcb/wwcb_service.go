// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wwcb

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/browser"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"

	"chromiumos/tast/services/cros/wwcb"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			wwcb.RegisterWWCBServiceServer(srv, &WWCBService{s: s})
		},
	})
}

// WWCBService implements tast.cros.wwcb.WWCBService.
type WWCBService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *WWCBService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	testing.ContextLog(ctx, "Remote Test New")

	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr

	return &empty.Empty{}, nil
}

func (c *WWCBService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Remote Test close")

	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

func (c *WWCBService) Echo(ctx context.Context, req *wwcb.StringRequest) (*wwcb.StringResponse, error) {
	testing.ContextLog(ctx, "Remote Test Echo")

	return &wwcb.StringResponse{ReturnString: "Echo: " + string(req.SendString)}, nil
}

func (c *WWCBService) Dock1PersistentStep3(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return nil, errors.Wrap(err, "failed to verify a connected external display")
	}
	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to verfiy display state")
	}
	return &empty.Empty{}, nil
}

func (c *WWCBService) Dock1PersistentStep4(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return nil, err
	}

	if _, err := browser.Launch(ctx, tconn, c.cr, "https://www.google.com"); err != nil {
		return nil, err
	}

	// Switch window to external display.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return err
		}
		for _, w := range ws {
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				return err
			}
			if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, true)(ctx); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to switch windows to external display")
	}
	return &empty.Empty{}, nil
}

func (c *WWCBService) VerifyAllWindowsOnDisplay(ctx context.Context, req *wwcb.BoolRequest) (*empty.Empty, error) {
	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, bool(req.SendBool)); err != nil {
		return nil, errors.Wrap(err, "failed to verify all windows on internal display")
	}
	return &empty.Empty{}, nil
}

func (c *WWCBService) Dock1PersistentStep6(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	infos, err := utils.GetInternalAndExternalDisplays(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos.Internal); err != nil {
		return nil, err
	}

	// Switch windows to internal display.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return err
		}
		for _, w := range ws {
			if err := w.ActivateWindow(ctx, tconn); err != nil {
				return err
			}
			if err := utils.SwitchWindowToDisplay(ctx, tconn, kb, false)(ctx); err != nil {
				return err
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 2 * time.Second}); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Change external display as primary display, then verify windows on external display")

	if err := utils.EnsureDisplayPrimary(ctx, tconn, &infos.External); err != nil {
		return nil, err
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, true); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (c *WWCBService) Dock1PersistentStep8(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	tconn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	intDispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if err := utils.EnsureDisplayPrimary(ctx, tconn, intDispInfo); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Enter mirror mode, then verify each display mirror source ID")

	if err := utils.SetMirrorDisplay(ctx, tconn, checked.True); err != nil {
		return nil, err
	}

	// Verify internal display MirroringSourceID and ID are the same.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		intDispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			return err
		}
		if intDispInfo.ID != intDispInfo.MirroringSourceID {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Exit mirror mode, then verify display state and windows are on internal display")

	if err := utils.SetMirrorDisplay(ctx, tconn, checked.False); err != nil {
		return nil, err
	}

	if err := utils.VerifyDisplayState(ctx, tconn); err != nil {
		return nil, err
	}

	if err := utils.VerifyAllWindowsOnDisplay(ctx, tconn, false); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
