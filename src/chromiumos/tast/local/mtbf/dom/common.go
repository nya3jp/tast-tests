// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dom

import (
	"fmt"
	"strings"
)

// Query generates querySelector() js string.
func Query(selector string) string {
	if strings.Contains(selector, "document") {
		return selector
	}
	return fmt.Sprintf("document.querySelector('%s')", selector)
}

// Click generates dom.click() js string.
func Click(selector string) string {
	return Query(selector) + ".click()"
}

// Focus generates dom.focus() js string.
func Focus(selector string) string {
	return Query(selector) + ".focus()"
}

// DispatchEvent generates dom.dispatchEvent() js string.
func DispatchEvent(selector, event string) string {
	return fmt.Sprintf("%s.dispatchEvent(%s)", Query(selector), event)
}

// AdvanceFindQuery generates complex query selector js string.
func AdvanceFindQuery(selector, arrowFunction string) string {
	return fmt.Sprintf("Array.from(document.querySelectorAll('%s')).find(%s)", selector, arrowFunction)
}
