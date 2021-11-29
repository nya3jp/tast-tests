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

	commoncros "chromiumos/tast/common/cros"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/services/cros/ui"
	svcdef "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var automationService AutomationService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			automationService = AutomationService{s: s, sharedObject: commoncros.SharedObjectsForServiceSingleton}
			svcdef.RegisterAutomationServiceServer(srv, &automationService)
		},
		GuaranteeCompatibility: true,
	})
}

// AutomationService implements tast.cros.ui.AutomationService
type AutomationService struct {
	s            *testing.ServiceState
	sharedObject *commoncros.SharedObjectsForService
}

func (svc *AutomationService) NodeInfo(ctx context.Context, req *svcdef.NodeInfoRequest) (*svcdef.NodeInfoResponse, error) {
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
		return nil, errors.Wrap(err, "Failed to get NodeInfo")
	}
	svcNodeInfo, _ := toServiceNodeInfo(nodeInfo)

	return &svcdef.NodeInfoResponse{
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

func (svc *AutomationService) LeftClick(ctx context.Context, req *svcdef.LeftClickRequest) (*empty.Empty, error) {
	if err := svc.click(ctx, leftClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (svc *AutomationService) RightClick(ctx context.Context, req *svcdef.RightClickRequest) (*empty.Empty, error) {
	if err := svc.click(ctx, rightClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
func (svc *AutomationService) DoubleClick(ctx context.Context, req *svcdef.DoubleClickRequest) (*empty.Empty, error) {
	if err := svc.click(ctx, doubleClick, req.Finder); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (svc *AutomationService) click(ctx context.Context, ct clickType, svcFinder *svcdef.Finder) error {
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

func (svc *AutomationService) IsNodeFound(ctx context.Context, req *svcdef.IsNodeFoundRequest) (*svcdef.IsNodeFoundResponse, error) {
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
		return nil, errors.Wrapf(err, "Error calling IsNodeFound with finder: %v", finder.Pretty())
	}
	return &svcdef.IsNodeFoundResponse{Found: found}, nil
}
func (svc *AutomationService) MouseClickAtLocation(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}
func (svc *AutomationService) WaitUntilExists(ctx context.Context, req *svcdef.WaitUntilExistsRequest) (*empty.Empty, error) {
	ui, err := getUIAutoContext(ctx, svc)
	if err != nil {
		return nil, err
	}
	finder, err := toFinder(req.Finder)
	if err != nil {
		return nil, err
	}
	// found, err := ui.WaitUntilExists(ctx, finder)
	if err := ui.WaitUntilExists(finder)(ctx); err != nil {
		return nil, errors.Wrapf(err, "Error calling IsNodeFound with finder: %v", finder.Pretty())
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
		return nil, errors.Wrap(err, "Failed to create test API connection")
	}
	ui := uiauto.New(tconn)
	return ui, nil
}

func toServiceNodeInfo(n *uiauto.NodeInfo) (*svcdef.NodeInfo, error) {
	return &svcdef.NodeInfo{
		ClassName: n.ClassName,
		Name:      n.Name,
		Value:     n.Value,
	}, nil
}

func toFinder(input *svcdef.Finder) (*nodewith.Finder, error) {
	// Create an Empty finder
	f := nodewith.Ancestor(nil)

	for idx, nw := range input.NodeWiths {
		switch val := nw.Value.(type) {
		case *ui.NodeWith_HasClass:
			f = f.HasClass(val.HasClass)
		case *ui.NodeWith_Name:
			f = f.Name(val.Name)
		case *ui.NodeWith_Role:
			r, _ := toRole(&val.Role)
			f = f.Role(r)
		case *ui.NodeWith_Nth:
			f = f.Nth(int(val.Nth))
		case *ui.NodeWith_AutofillAvailable:
			f = f.AutofillAvailable()
		case *ui.NodeWith_Collapsed:
			f = f.Collapsed()
		case *ui.NodeWith_Default:
			f = f.Default()
		case *ui.NodeWith_Editable:
			f = f.Editable()
		case *ui.NodeWith_Expanded:
			f = f.Expanded()
		case *ui.NodeWith_Focusable:
			f = f.Focusable()
		case *ui.NodeWith_Focused:
			f = f.Focused()
		case *ui.NodeWith_Horizontal:
			f = f.Horizontal()
		case *ui.NodeWith_Hovered:
			f = f.Hovered()
		case *ui.NodeWith_Ignored:
			f = f.Ignored()
		case *ui.NodeWith_Invisible:
			f = f.Invisible()
		case *ui.NodeWith_Linked:
			f = f.Linked()
		case *ui.NodeWith_Multiline:
			f = f.Multiline()
		case *ui.NodeWith_Multiselectable:
			f = f.Multiselectable()
		case *ui.NodeWith_Offscreen:
			f = f.Offscreen()
		case *ui.NodeWith_Protected:
			f = f.Protected()
		case *ui.NodeWith_Required:
			f = f.Required()
		case *ui.NodeWith_RichlyEditable:
			f = f.RichlyEditable()
		case *ui.NodeWith_Vertical:
			f = f.Vertical()
		case *ui.NodeWith_Visited:
			f = f.Visited()
		case *ui.NodeWith_Visible:
			f = f.Visible()
		case *ui.NodeWith_Onscreen:
			f = f.Onscreen()
		case *ui.NodeWith_State:
			//TODO (jonfan): consider just using individual API.
			f = f.State(state.Default, val.State.Value)
		case *ui.NodeWith_NameRegex:
			f = f.NameRegex(regexp.MustCompile(val.NameRegex))
		case *ui.NodeWith_NameStartingWith:
			f = f.NameStartingWith(val.NameStartingWith)
		case *ui.NodeWith_NameContaining:
			f = f.NameContaining(val.NameContaining)
		case *ui.NodeWith_Ancestor:
			ancestor, err := toFinder(val.Ancestor)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed when calling toFinder() on ancestor for %v", ancestor)
			}
			f = f.Ancestor(ancestor)
		case *ui.NodeWith_First:
			f = f.First()
		case *ui.NodeWith_Root:
			if idx != 0 || len(input.NodeWiths) > 1 {
				return nil, errors.New("Root can only be the only nodewith predicate")
			}
			f = nodewith.Root()
		}
	}
	return f, nil

}

func toRole(input *svcdef.Role) (role.Role, error) {
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
