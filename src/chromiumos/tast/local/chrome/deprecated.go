// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
)

// NewConn starts a new session using sm for communicating with the supplied target.
// pageURL is only used when logging JavaScript console messages via lm.
//
// DEPRECATED: Do not call NewConn from outside the chrome package.
func NewConn(ctx context.Context, s *cdputil.Session, id target.ID, la *jslog.Aggregator, pageURL string, chromeErr func(error) error) (c *Conn, retErr error) {
	return driver.NewConn(ctx, s, id, la, pageURL, chromeErr)
}
