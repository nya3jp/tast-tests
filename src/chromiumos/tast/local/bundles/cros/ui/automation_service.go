// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var automationService AutomationService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			automationService = AutomationService{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterAutomationServiceServer(srv, &automationService)
		},
		GuaranteeCompatibility: true,
	})
}

// AutomationService implements tast.cros.ui.AutomationService
type AutomationService struct {
	sharedObject *common.SharedObjectsForService
}

// Info returns the information for the node found by the input finder.
func (svc *AutomationService) Info(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := ui.Info(ctx, finder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get NodeInfo")
	}
	pbNodeInfo, err := toNodeInfoPB(nodeInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed calling toNodeInfoPB")
	}

	return &pb.InfoResponse{
		NodeInfo: pbNodeInfo,
	}, nil
}

// clickType describes how user clicks mouse.
type clickType int

const (
	leftClick clickType = iota
	rightClick
	doubleClick
)

// LeftClick clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) LeftClick(ctx context.Context, req *pb.LeftClickRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if err := svc.click(ctx, leftClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// RightClick clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) RightClick(ctx context.Context, req *pb.RightClickRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if err := svc.click(ctx, rightClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// DoubleClick clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) DoubleClick(ctx context.Context, req *pb.DoubleClickRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if err := svc.click(ctx, doubleClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// click triggers a click based on clicktype on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) click(ctx context.Context, ct clickType, svcFinder *pb.Finder) error {
	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return err
	}
	finder, err := toFinder(svcFinder)
	if err != nil {
		return err
	}
	switch ct {
	case leftClick:
		return ui.LeftClick(finder)(ctx)
	case rightClick:
		return ui.RightClick(finder)(ctx)
	case doubleClick:
		return ui.DoubleClick(finder)(ctx)
	default:
		return errors.New("invalid click type")
	}
}

// IsNodeFound immediately checks if any nodes found with given finder.
// It returns true if found otherwise false.
func (svc *AutomationService) IsNodeFound(ctx context.Context, req *pb.IsNodeFoundRequest) (*pb.IsNodeFoundResponse, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}
	found, err := ui.IsNodeFound(ctx, finder)
	if err != nil {
		return nil, errors.Wrapf(err, "failed calling IsNodeFound with finder: %v", finder.Pretty())
	}
	return &pb.IsNodeFoundResponse{Found: found}, nil
}

// MouseClickAtLocation clicks on the specified location.
func (svc *AutomationService) MouseClickAtLocation(ctx context.Context, req *pb.MouseClickAtLocationRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	loc := toPoint(req.Point)

	// uiauto.clickType is private. Passing an int type variable does not compile whereas integer literal compiles ok.
	switch req.ClickType {
	case pb.ClickType_CLICK_TYPE_LEFT_CLICK:
		err = ui.MouseClickAtLocation(0, loc)(ctx)
	case pb.ClickType_CLICK_TYPE_RIGHT_CLICK:
		err = ui.MouseClickAtLocation(1, loc)(ctx)
	case pb.ClickType_CLICK_TYPE_DOUBLE_CLICK:
		err = ui.MouseClickAtLocation(2, loc)(ctx)
	default:
		return nil, errors.New("unknown clicktype")
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed calling MouseClickAtLocation with clicktype: %v and location: %v", req.ClickType, loc)
	}

	return &empty.Empty{}, nil
}

// WaitUntilExists waits until the node found by the input finder exists.
func (svc *AutomationService) WaitUntilExists(ctx context.Context, req *pb.WaitUntilExistsRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}
	if err := ui.WaitUntilExists(finder)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed calling WaitUntilExists with finder: %v", finder.Pretty())
	}
	return &empty.Empty{}, nil
}

func mouseButton(button pb.MouseButton) (mouse.Button, error) {
	switch button {
	case pb.MouseButton_LEFT_BUTTON:
		return mouse.LeftButton, nil
	case pb.MouseButton_RIGHT_BUTTON:
		return mouse.RightButton, nil
	case pb.MouseButton_MIDDLE_BUTTON:
		return mouse.MiddleButton, nil
	case pb.MouseButton_BACK_BUTTON:
		return mouse.BackButton, nil
	case pb.MouseButton_FORWARD_BUTTON:
		return mouse.ForwardButton, nil
	}
	return mouse.LeftButton, errors.Errorf("unsupported mouse button %d", button)
}

// MousePress left clicks and holds on the node. The press needs to be released by caller.
func (svc *AutomationService) MousePress(ctx context.Context, req *pb.MousePressRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}
	button, err := mouseButton(req.MouseButton)
	if err != nil {
		return nil, err
	}
	if err := ui.MousePress(button, finder)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed calling MousePress on button %q with finder: %v", string(button), finder.Pretty())
	}
	return &empty.Empty{}, nil
}

// MouseMoveTo moves the mouse to hover the requested node.
func (svc *AutomationService) MouseMoveTo(ctx context.Context, req *pb.MouseMoveToRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}

	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}

	if err := ui.MouseMoveTo(finder, time.Duration(req.DurationMs)*time.Millisecond)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed moving mouse to finder %v", finder)
	}
	return &empty.Empty{}, nil
}

// MouseRelease releases left click.
func (svc *AutomationService) MouseRelease(ctx context.Context, req *pb.MouseReleaseRequest) (*empty.Empty, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	button, err := mouseButton(req.MouseButton)
	if err != nil {
		return nil, err
	}
	if err := ui.MouseRelease(button)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed calling MouseRelease on button %q", string(button))
	}
	return &empty.Empty{}, nil
}

// CaptureScreenshot captures the screenshot of the whole screen or a stable UI node.
func (svc *AutomationService) CaptureScreenshot(ctx context.Context, req *pb.CaptureScreenshotRequest) (*pb.CaptureScreenshotResponse, error) {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}

	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}

	var img image.Image
	if req.Finder != nil {
		finder, err := toFinder(req.Finder)
		if err != nil {
			return nil, err
		}
		nodeInfo, err := ui.Info(ctx, finder)
		if err != nil {
			return nil, err
		}
		img, err = screenshot.GrabAndCropScreenshot(ctx, cr, nodeInfo.Location)
	} else {
		img, err = screenshot.GrabScreenshot(ctx, cr)
	}
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	if err = png.Encode(&b, img); err != nil {
		return nil, err
	}

	return &pb.CaptureScreenshotResponse{
		PngBase64: b.Bytes(),
	}, nil
}

func getUIAutoContext(ctx context.Context, svc *AutomationService) (*uiauto.Context, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}

	// When in OOBE, use SigninProfileTestAPIConn to create the test connection.
	var tconn *chrome.TestConn
	var err error
	if cr.LoginMode() == "NoLogin" {
		tconn, err = cr.SigninProfileTestAPIConn(ctx)
	} else {
		tconn, err = cr.TestAPIConn(ctx)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	ui := uiauto.New(tconn)
	return ui, nil
}

func toPoint(p *pb.Point) coords.Point {
	return coords.Point{
		X: int(p.X),
		Y: int(p.Y),
	}
}

func toNodeInfoPB(n *uiauto.NodeInfo) (*pb.NodeInfo, error) {
	return &pb.NodeInfo{
		Checked:        toCheckedPB(n.Checked),
		ClassName:      n.ClassName,
		HtmlAttributes: n.HTMLAttributes,
		Location:       toRectPB(n.Location),
		Name:           n.Name,
		Restriction:    toRestrictionPB(n.Restriction),
		Role:           toRolePB(n.Role),
		State:          toStateMap(n.State),
		Value:          n.Value,
	}, nil
}

func toRolePB(r role.Role) pb.Role {
	enumVal := pb.Role_value["ROLE_"+toConstCase(string(r))]
	r2 := pb.Role(enumVal)
	return r2
}

func toRole(input pb.Role) (role.Role, error) {
	roleConstantCase := input.Enum().String()
	//Trim Role_ prefix
	roleConstantCase = roleConstantCase[5:]
	return role.Role(toCamelCase(roleConstantCase)), nil
}

func toStateMap(s map[state.State]bool) map[string]bool {
	m := make(map[string]bool)
	for k, v := range s {
		m[string(k)] = v
	}
	return m
}

func toRectPB(r coords.Rect) *pb.Rect {
	return &pb.Rect{
		Left:   int32(r.Left),
		Top:    int32(r.Top),
		Width:  int32(r.Width),
		Height: int32(r.Height),
	}
}

func toCheckedPB(c checked.Checked) pb.Checked {
	switch c {
	case checked.True:
		return pb.Checked_CHECKED_TRUE
	case checked.False:
		return pb.Checked_CHECKED_FALSE
	case checked.Mixed:
		return pb.Checked_CHECKED_MIXED
	default:
		return pb.Checked_CHECKED_UNSPECIFIED
	}
}

func toRestrictionPB(r restriction.Restriction) pb.Restriction {
	switch r {
	case restriction.Disabled:
		return pb.Restriction_RESTRICTION_DISABLED
	case restriction.ReadOnly:
		return pb.Restriction_RESTRICTION_READ_ONLY
	case restriction.None:
		return pb.Restriction_RESTRICTION_NONE
	default:
		return pb.Restriction_RESTRICTION_UNSPECIFIED
	}
}

func toFinder(input *pb.Finder) (*nodewith.Finder, error) {
	// Create an Empty finder
	f := nodewith.Ancestor(nil)

	for idx, nw := range input.NodeWiths {
		switch val := nw.Value.(type) {
		case *pb.NodeWith_HasClass:
			f = f.HasClass(val.HasClass)
		case *pb.NodeWith_Name:
			f = f.Name(val.Name)
		case *pb.NodeWith_Role:
			r, _ := toRole(val.Role)
			f = f.Role(r)
		case *pb.NodeWith_Nth:
			f = f.Nth(int(val.Nth))
		case *pb.NodeWith_AutofillAvailable:
			f = f.AutofillAvailable()
		case *pb.NodeWith_Collapsed:
			f = f.Collapsed()
		case *pb.NodeWith_IsDefault:
			f = f.Default()
		case *pb.NodeWith_Editable:
			f = f.Editable()
		case *pb.NodeWith_Expanded:
			f = f.Expanded()
		case *pb.NodeWith_Focusable:
			f = f.Focusable()
		case *pb.NodeWith_Focused:
			f = f.Focused()
		case *pb.NodeWith_Horizontal:
			f = f.Horizontal()
		case *pb.NodeWith_Hovered:
			f = f.Hovered()
		case *pb.NodeWith_Ignored:
			f = f.Ignored()
		case *pb.NodeWith_Invisible:
			f = f.Invisible()
		case *pb.NodeWith_Linked:
			f = f.Linked()
		case *pb.NodeWith_Multiline:
			f = f.Multiline()
		case *pb.NodeWith_Multiselectable:
			f = f.Multiselectable()
		case *pb.NodeWith_Offscreen:
			f = f.Offscreen()
		case *pb.NodeWith_IsProtected:
			f = f.Protected()
		case *pb.NodeWith_Required:
			f = f.Required()
		case *pb.NodeWith_RichlyEditable:
			f = f.RichlyEditable()
		case *pb.NodeWith_Vertical:
			f = f.Vertical()
		case *pb.NodeWith_Visited:
			f = f.Visited()
		case *pb.NodeWith_Visible:
			f = f.Visible()
		case *pb.NodeWith_Onscreen:
			f = f.Onscreen()
		case *pb.NodeWith_State:
			// TODO(jonfan): The syntax on State gRPC APIs is really clunky.
			// Can we instead rely solely on more descriptive individual APIs
			// like Invisible() and Visible()?
			f = f.State(state.Default, val.State.Value)
		case *pb.NodeWith_NameRegex:
			f = f.NameRegex(regexp.MustCompile(val.NameRegex))
		case *pb.NodeWith_NameStartingWith:
			f = f.NameStartingWith(val.NameStartingWith)
		case *pb.NodeWith_NameContaining:
			f = f.NameContaining(val.NameContaining)
		case *pb.NodeWith_Ancestor:
			ancestor, err := toFinder(val.Ancestor)
			if err != nil {
				return nil, errors.Wrapf(err, "failed when calling toFinder() on ancestor for %v", ancestor)
			}
			f = f.Ancestor(ancestor)
		case *pb.NodeWith_First:
			f = f.First()
		case *pb.NodeWith_Root:
			if idx != 0 || len(input.NodeWiths) > 1 {
				return nil, errors.New("Root can only be the only nodewith predicate")
			}
			f = nodewith.Root()
		}
	}
	return f, nil

}

func toCamelCase(constantCase string) string {
	var s []string
	for i, token := range strings.Split(constantCase, "_") {
		if i == 0 {
			s = append(s, strings.ToLower(token))
		} else if i >= 1 {
			s = append(s, strings.ToUpper(token[0:1])+strings.ToLower(token[1:]))
		}
	}
	return strings.Join(s, "")
}

func toConstCase(camelCase string) string {
	re := regexp.MustCompile(`([A-Z])`)
	return strings.ToUpper(re.ReplaceAllString(camelCase, `_$1`))
}
