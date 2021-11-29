// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	common "chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var automationService AutomationService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			automationService = AutomationService{s: s, sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterAutomationServiceServer(srv, &automationService)
		},
		GuaranteeCompatibility: true,
	})
}

// AutomationService implements tast.cros.ui.AutomationService
type AutomationService struct {
	s            *testing.ServiceState
	sharedObject *common.SharedObjectsForService
}

// Info returns the information for the node found by the input finder.
//TODO(jonfan): Expose full Info structure
func (svc *AutomationService) Info(ctx context.Context, req *pb.InfoRequest) (*pb.InfoResponse, error) {
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
	svcNodeInfo, _ := toServiceNodeInfo(nodeInfo)

	return &pb.InfoResponse{
		NodeInfo: svcNodeInfo,
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
	if err := svc.click(ctx, leftClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// RightClick clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) RightClick(ctx context.Context, req *pb.RightClickRequest) (*empty.Empty, error) {
	if err := svc.click(ctx, rightClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// DoubleClick clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
func (svc *AutomationService) DoubleClick(ctx context.Context, req *pb.DoubleClickRequest) (*empty.Empty, error) {
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
//TODO(jonfan): Click at location. Testing:  Use Info and this to replace one of the clicks
func (svc *AutomationService) MouseClickAtLocation(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
func (svc *AutomationService) WaitUntilExists(ctx context.Context, req *pb.WaitUntilExistsRequest) (*empty.Empty, error) {
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

func getUIAutoContext(ctx context.Context, svc *AutomationService) (*uiauto.Context, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}
	ui := uiauto.New(tconn)
	return ui, nil
}

func toServiceNodeInfo(n *uiauto.NodeInfo) (*pb.NodeInfo, error) {
	return &pb.NodeInfo{
		ClassName: n.ClassName,
		Name:      n.Name,
		Value:     n.Value,
	}, nil
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
			r, _ := toRole(&val.Role)
			f = f.Role(r)
		case *pb.NodeWith_Nth:
			f = f.Nth(int(val.Nth))
		case *pb.NodeWith_AutofillAvailable:
			f = f.AutofillAvailable()
		case *pb.NodeWith_Collapsed:
			f = f.Collapsed()
		case *pb.NodeWith_Default:
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
		case *pb.NodeWith_Protected:
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
			//TODO (jonfan): The syntax on State gRPC APIs is really clunky.
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

func toRole(input *pb.Role) (role.Role, error) {
	roleConstantCase := input.Enum().String()
	return role.Role(toCamelCase(roleConstantCase[5:])), nil
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
