// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WaitUntilAllTabsLoaded checks the status of all tabs in the current browser
// window and waits until all tabs are not 'loading' status.
func WaitUntilAllTabsLoaded(ctx context.Context, c *Conn, timeout time.Duration) error {
	query := map[string]interface{}{
		"status":        "loading",
		"currentWindow": true,
	}
	queryData, err := json.Marshal(query)
	if err != nil {
		return errors.Wrap(err, "failed to marshal query")
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.tabs.query)(%s)`, string(queryData))
	return testing.Poll(ctx, func(ctx context.Context) error {
		var tabs []map[string]interface{}
		if err := c.EvalPromise(ctx, expr, &tabs); err != nil {
			return testing.PollBreak(err)
		}
		if len(tabs) == 0 {
			return nil
		}
		return errors.Errorf("still %d tabs are loading", len(tabs))
	}, &testing.PollOptions{Timeout: timeout})
}
