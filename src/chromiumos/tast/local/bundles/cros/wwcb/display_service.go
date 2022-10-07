// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/input"
	"chromiumos/tast/services/cros/wwcb"
	"chromiumos/tast/testing"
)

const displayTimeout = 30 * time.Second

func init() {
	var displayService DisplayService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			displayService = DisplayService{sharedObject: common.SharedObjectsForServiceSingleton}
			wwcb.RegisterDisplayServiceServer(srv, &displayService)
		},
		GuaranteeCompatibility: true,
	})
}

// DisplayService implements tast.cros.wwcb.DisplayService.
type DisplayService struct {
	sharedObject *common.SharedObjectsForService
}

// SetMirrorDisplay enables or disables mirror mode in display settings.
func (ds *DisplayService) SetMirrorDisplay(ctx context.Context, req *wwcb.QueryRequest) (*empty.Empty, error) {
	cr := ds.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	// Set mirror display.
	var want checked.Checked
	if bool(req.Enable) {
		want = checked.True
	} else {
		want = checked.False
	}
	if err := utils.SetMirrorDisplay(ctx, tconn, want); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to set mirror display")
	}

	// Expect the display is changed. Return err after poll timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		intDispInfo, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display infos in mirror mode")
		}

		if bool(req.Enable) {
			if intDispInfo.ID != intDispInfo.MirroringSourceID {
				return errors.Errorf("unexcepted mirror source ID: got %s, want %s", intDispInfo.MirroringSourceID, intDispInfo.ID)
			}
		} else {
			if intDispInfo.ID == intDispInfo.MirroringSourceID {
				return errors.New("mirror source ID should be empty string")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return &empty.Empty{}, err
	}

	return &empty.Empty{}, nil
}

// SetPrimaryDisplay sets the internal or external display as primary mode.
func (ds *DisplayService) SetPrimaryDisplay(ctx context.Context, req *wwcb.QueryRequest) (*empty.Empty, error) {
	cr := ds.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return &empty.Empty{}, err
	}

	if int(req.DisplayIndex) >= len(infos) {
		return &empty.Empty{}, errors.New("display index is out of range")
	}

	// Set the display to primary.
	displayID := infos[int(req.DisplayIndex)].ID
	isPrimary := true
	if err := display.SetDisplayProperties(ctx, tconn, displayID, display.DisplayProperties{IsPrimary: &isPrimary}); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to set display properties")
	}

	// Expect the display is changed. Return err after poll timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get primary display info")
		}
		if primaryInfo.ID != displayID {
			return errors.New("unable to set display as primary")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return &empty.Empty{}, err
	}

	return &empty.Empty{}, nil
}

// SwitchWindowToDisplay finds the window with given title then switch it to the expected display.
func (ds *DisplayService) SwitchWindowToDisplay(ctx context.Context, req *wwcb.QueryRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	cr := ds.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.Title == string(req.WindowTitle)
	})
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to find window")
	}

	if err := w.ActivateWindow(ctx, tconn); err != nil {
		return &empty.Empty{}, err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	}

	if int(req.DisplayIndex) >= len(infos) {
		return &empty.Empty{}, errors.New("display index is out of range")
	}

	// If unable to get expected window, then switch it.
	ui := uiauto.New(tconn)
	displayName := infos[int(req.DisplayIndex)].Name
	expectedRootWindow := nodewith.Name(displayName).Role(role.Window)
	currentWindow := nodewith.Name(w.Title).Role(role.Window)
	expectedWindow := currentWindow.Ancestor(expectedRootWindow).First()
	if err := ui.Exists(expectedWindow)(ctx); err != nil {
		testing.ContextLog(ctx, "Expected window not found: ", err)
		testing.ContextLogf(ctx, "Switch window %q to %s", w.Title, displayName)
		return &empty.Empty{}, uiauto.Combine("switch window to "+displayName,
			kb.AccelAction("Search+Alt+M"),
			ui.WithTimeout(5*time.Second).WaitUntilExists(expectedWindow),
		)(ctx)
	}

	return &empty.Empty{}, nil
}

// VerifyWindowOnDisplay finds the window with given title then verify it is showing on the expected display or not.
func (ds *DisplayService) VerifyWindowOnDisplay(ctx context.Context, req *wwcb.QueryRequest) (*empty.Empty, error) {
	cr := ds.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, err
	}

	// Retries to check window & display info, cause system might need time to apply the changes.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.Title == string(req.WindowTitle)
		})
		if err != nil {
			return errors.Wrap(err, "failed to find window")
		}

		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		if int(req.DisplayIndex) >= len(infos) {
			return errors.New("display index is out of range")
		}

		displayID := infos[int(req.DisplayIndex)].ID
		if w.DisplayID != displayID && w.IsVisible && w.IsFrameVisible {
			return errors.Errorf("window isn't showing on certain display: got %s, want %s", w.DisplayID, displayID)
		}

		return nil
	}, &testing.PollOptions{Timeout: displayTimeout}); err != nil {
		return &empty.Empty{}, err
	}

	return &empty.Empty{}, nil
}

// VerifyDisplayCount verifies the given  display count to compare with the current numbers of display that system detected.
func (ds *DisplayService) VerifyDisplayCount(ctx context.Context, req *wwcb.QueryRequest) (*empty.Empty, error) {
	cr := ds.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, err
	}

	// Retries to check window & display info, cause system might need time to apply the changes.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display info")
		}

		if len(infos) != int(req.DisplayCount) {
			return errors.Errorf("unexpected number of displays: got %d, want %d", len(infos), int(req.DisplayCount))
		}
		return nil
	}, &testing.PollOptions{Timeout: displayTimeout}); err != nil {
		return &empty.Empty{}, err
	}

	return &empty.Empty{}, nil
}
