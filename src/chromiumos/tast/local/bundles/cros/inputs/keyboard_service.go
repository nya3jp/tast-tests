// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	commoncros "chromiumos/tast/common/cros"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
)

func init() {

	var keyboardService KeyboardService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			keyboardService = KeyboardService{s: s, sharedObject: commoncros.SharedObjectsForServiceSingleton}
			inputs.RegisterKeyboardServiceServer(srv, &keyboardService)
		},
		GuaranteeCompatibility: true,
	})
}

// KeyboardService implements tast.cros.inputs.KeyboardService.
type KeyboardService struct {
	s            *testing.ServiceState
	sharedObject *commoncros.SharedObjectsForService
}

// Type allows uses to input keystrokes
func (svc *KeyboardService) Type(ctx context.Context, req *inputs.TypeRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "KeyboardService LOG 4")
	//TODO(jonfan): What precondition check is needed before using chrome
	cr := svc.sharedObject.Chrome

	if _, err := cr.NewConn(ctx, fmt.Sprintf("https://google.com/search?q=%s", req.Key)); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
