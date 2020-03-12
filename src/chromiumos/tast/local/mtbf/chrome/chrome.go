// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"net/url"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
)

// NewConn creates a new Chrome renderer and returns a connection to it.
func NewConn(ctx context.Context, cr *chrome.Chrome, urlPath string) (*chrome.Conn, error) {
	u, err := url.Parse(urlPath)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeUnknownURL, err, urlPath)
	}

	conn, err := cr.NewConn(ctx, urlPath)
	if err != nil {
		if u.Scheme == "chrome" || u.Scheme == "about" {
			return nil, mtbferrors.New(mtbferrors.ChromeOpenInURL, err, urlPath)
		}
		return nil, mtbferrors.New(mtbferrors.ChromeOpenExtURL, err, urlPath)
	}

	return conn, nil
}
