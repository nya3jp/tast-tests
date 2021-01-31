// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/extension"
)

// ComputeExtensionID computes the 32-character ID that Chrome will use for an unpacked
// extension in dir. The extension's manifest file must contain the "key" field.
// Use the following command to generate a new key:
//  openssl genrsa 2048 | openssl rsa -pubout -outform der | openssl base64 -A
func ComputeExtensionID(dir string) (string, error) {
	return extension.ComputeExtensionID(dir)
}

// AddTastLibrary introduces tast library into the page for the given conn.
// This introduces a variable named "tast" to its scope, and it is the
// caller's responsibility to avoid the conflict.
func AddTastLibrary(ctx context.Context, conn *Conn) error {
	// Ensure the page is loaded so the tast library will be added properly.
	if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return errors.Wrap(err, "failed waiting for page to load")
	}
	return conn.Eval(ctx, extension.TastLibraryJS, nil)
}

// ExtensionBackgroundPageURL returns the URL to the background page for
// the extension with the supplied ID.
func ExtensionBackgroundPageURL(extID string) string {
	return extension.BackgroundPageURL(extID)
}
