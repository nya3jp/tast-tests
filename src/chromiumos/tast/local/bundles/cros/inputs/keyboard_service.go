// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	commoncros "chromiumos/tast/common/cros"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
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

// Type injects key events suitable for generating the string s.
// Only characters that can be typed using a QWERTY keyboard are supported,
// and the current keyboard layout must be QWERTY. The left Shift key is automatically
// pressed and released for uppercase letters or other characters that can be typed
// using Shift.
func (svc *KeyboardService) Type(ctx context.Context, req *inputs.TypeRequest) (*empty.Empty, error) {
	// For simplicity, keyboard are created and destroy per method call.
	//TODO(jonfan): Decide if keyboard should be a shared object.
	// Considerations: how expansive is it to get and destroy keyboard every single time?
	// If it stores as shared objects, should end users of the gRPC service be in charge of the
	// lifecycle management of keyboard.

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get keyboard handle")
	}
	defer kb.Close()
	if err := kb.Type(ctx, req.Key); err != nil {
		return nil, errors.Wrapf(err, "Failed to type %v", req.Key)
	}

	return &empty.Empty{}, nil
}

// Accel injects a sequence of key events simulating the accelerator (a.k.a. hotkey) described by s being typed.
// Accelerators are described as a sequence of '+'-separated, case-insensitive key characters or names.
// In addition to non-whitespace characters that are present on a QWERTY keyboard, the following key names may be used:
//	Modifiers:     "Ctrl", "Alt", "Search", "Shift"
//	Whitespace:    "Enter", "Space", "Tab", "Backspace"
//	Function keys: "F1", "F2", ..., "F12"
// "Shift" must be included for keys that are typed using Shift; for example, use "Ctrl+Shift+/" rather than "Ctrl+?".
func (svc *KeyboardService) Accel(ctx context.Context, req *inputs.AccelRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get keyboard handle")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, req.Key); err != nil {
		return nil, errors.Wrapf(err, "Failed to call Accel %v", req.Key)
	}
	return &empty.Empty{}, nil
}

// AccelPress injects a sequence of key events simulating pressing the accelerator (a.k.a. hotkey) described by s.
func (svc *KeyboardService) AccelPress(ctx context.Context, req *inputs.AccelPressRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get keyboard handle")
	}
	defer kb.Close()
	if err := kb.AccelPress(ctx, req.Key); err != nil {
		return nil, errors.Wrapf(err, "Failed to call AccelPress %v", req.Key)
	}
	return &empty.Empty{}, nil
}

// AccelRelease injects a sequence of key events simulating release the accelerator (a.k.a. hotkey) described by s.
func (svc *KeyboardService) AccelRelease(ctx context.Context, req *inputs.AccelReleaseRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get keyboard handle")
	}
	defer kb.Close()
	if err := kb.AccelRelease(ctx, req.Key); err != nil {
		return nil, errors.Wrapf(err, "Failed to call AccelRelease %v", req.Key)
	}
	return &empty.Empty{}, nil
}

// TypeSequence injects key events suitable given a string slice seq, where seq
// is a combination of rune keys and accelerators.
// For each string s, it uses Type() to inject a key event if the len(s) = 1,
// and uses Accel() to inject a key event if the len(s) > 1.
// E.g., when calling TypeSequence({"S","e","q","space"}), it calls
// Type("S"), Type("e"), Type("q") and Accel("space") respectively.
// func (svc *KeyboardService) TypeSequence(ctx context.Context, req *inputs.TypeSequenceRequest) (*empty.Empty, error) {
// 	kb, err := input.Keyboard(ctx)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "Failed to get keyboard handle")
// 	}
// 	defer kb.Close()
// 	if err := kb.TypeSequence(ctx, req.Sequence); err != nil {
// 		return nil, errors.Wrapf(err, "Failed to call TypeSequence %v", req.Sequence)
// 	}
// 	return &empty.Empty{}, nil
// }
