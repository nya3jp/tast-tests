// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package loginstatus facilitates using the Autotest API function loginStatus.
package loginstatus

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// LoginStatus represents LoginStatusDict in the chromium code base, in
// chrome/common/extensions/api/autotest_private.idl
type LoginStatus struct {
	IsLoggedIn         bool `json:"isLoggedIn"`
	IsOwner            bool `json:"isOwner"`
	IsScreenLocked     bool `json:"isScreenLocked"`
	IsReadyForPassword bool `json:"isReadyForPassword"`

	IsRegularUser *bool `json:"isRegularUser,omitempty"`
	IsGuest       *bool `json:"isGuest,omitempty"`
	IsKiosk       *bool `json:"isKiosk,omitempty"`

	Email               *string `json:"email,omitempty"`
	DisplayEmail        *string `json:"displayEmail,omitempty"`
	DisplayName         *string `json:"displayName,omitempty"`
	UserImage           *string `json:"userImage,omitempty"`
	HasValidOauth2Token *bool   `json:"hasValidOauth2Token,omitempty"`
}

// GetLoginStatus gets a LoginStatus from the Autotest API function loginStatus.
func GetLoginStatus(ctx context.Context, tconn *chrome.TestConn) (*LoginStatus, error) {
	var status LoginStatus
	if err := tconn.Call(ctx, &status, "tast.promisify(chrome.autotestPrivate.loginStatus)"); err != nil {
		return nil, errors.Wrap(err, "failed to invoke Autotest API function loginStatus")
	}
	return &status, nil
}
