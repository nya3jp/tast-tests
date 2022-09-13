// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

const clipboardHistoryTextItemViewClassName = "ClipboardHistoryTextItemView"

// FindFirstTextItem returns a finder which locates the first text item in the
// clipboard history menu.
func FindFirstTextItem() *nodewith.Finder {
	return nodewith.ClassName(clipboardHistoryTextItemViewClassName).First()
}
