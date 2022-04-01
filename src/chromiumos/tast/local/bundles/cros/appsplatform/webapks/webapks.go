// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webapks defines test webAPKs used in appsplatform tests.
// It is important that webAPKs ports are distinct.
// This will guarantee, they will not clash, when used
// together in the same test.
package webapks

import (
	"chromiumos/tast/local/webapk"
)

// WebShareTargetWebApk app helps test sharing data to a web app.
var WebShareTargetWebApk = webapk.WebAPK{
	Name:              "Web Share Target Test App",
	ID:                "elcejdjmpnnkghnpldcjkafeoaadlkba",
	Port:              8000,
	ApkDataPath:       "WebShareTargetTestWebApk_20210707.apk",
	IndexPageDataPath: "webshare_index.html",
}
