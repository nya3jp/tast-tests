// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common provides common functionalities and utilities
package common

import (
	"context"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// SharedObjectsForService allows services to shared states of important objects, such as
// chrome and arc. While this provides access to the important objects, the lifecycle management
// of these objects is not the responsibility of this struct. Instead individual services
// will share the responsibility of managing the lifecycle of these objects.
// A common pattern is to include a reference during Service instantiation and registration. e.g.
//  testing.AddService(&testing.Service{
//	  Register: func(srv *grpc.Server, s *testing.ServiceState) {
//			automationService := AutomationService{s: s, sharedObject: common.SharedObjectsForServiceSingleton}
//			pb.RegisterAutomationServiceServer(srv, &automationService)
//		},
//	})
type SharedObjectsForService struct {
	Chrome *chrome.Chrome
	// Mutex to protect against concurrent access to Chrome
	ChromeMutex sync.Mutex
}

// UseTconn performs an action that requires access to tconn.
func UseTconn[T any](ctx context.Context, so *SharedObjectsForService, fn func(tconn *chrome.TestConn) (*T, error)) (*T, error) {
	so.ChromeMutex.Lock()
	defer so.ChromeMutex.Unlock()

	cr := so.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	return fn(tconn)
}

// SharedObjectsForServiceSingleton is the Singleton object that allows sharing states
// between services
var SharedObjectsForServiceSingleton = &SharedObjectsForService{}
