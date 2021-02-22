// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package storage contains helper functions to test different FilesApp storage
// folders that are shared from Chrome OS to ARC (e.g. Drive FS, Downloads).
// They can be used to check whether a file put in a Chrome OS folder can be
// read by Android apps, or to check whether a file pushed to an Android folder
// appears in the corresponding Chrome OS folder.
package storage
