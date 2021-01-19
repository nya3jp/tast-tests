// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
)

// Files manages local files of extensions to be installed to Chrome for
// testing.
type Files struct {
	user         *testExtension
	signin       *testExtension
	extraExtDirs []string
}

// PrepareExtensions writes test extensions to the local disk.
// The user test extension is always created. If signinExtensionKey is a
// non-empty string, the sign-in profile test extension is also created using
// the key. extraExtDirs specifies directories of extra extensions to be
// installed.
func PrepareExtensions(extraExtDirs []string, signinExtensionKey string) (files *Files, retErr error) {
	// Prepare the user test extension.
	user, err := prepareTestExtension(testExtensionKey, TestExtensionID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			user.RemoveAll()
		}
	}()

	// Prepare the sign-in profile test extension if it is available.
	var signin *testExtension
	if signinExtensionKey != "" {
		signin, err = prepareTestExtension(signinExtensionKey, SigninProfileTestExtensionID)
		if err != nil {
			return nil, err
		}
		defer func() {
			if retErr != nil {
				signin.RemoveAll()
			}
		}()
	}

	// Prepare extra extensions.
	for _, dir := range extraExtDirs {
		manifest := filepath.Join(dir, "manifest.json")
		if _, err = os.Stat(manifest); err != nil {
			return nil, errors.Wrap(err, "missing extension manifest")
		}
		if err := ChownContentsToChrome(dir); err != nil {
			return nil, err
		}
	}

	return &Files{
		user:         user,
		signin:       signin,
		extraExtDirs: extraExtDirs,
	}, nil
}

// DeprecatedDirs returns a list of directories where extensions are available.
//
// DEPRECATED: Use ChromeArgs instead. This method does not handle sign-in
// profile extensions correctly.
func (f *Files) DeprecatedDirs() []string {
	return append([]string{f.user.Dir()}, f.extraExtDirs...)
}

// ChromeArgs returns a list of arguments to be passed to Chrome to enable
// extensions.
func (f *Files) ChromeArgs() []string {
	extDirs := append([]string{f.user.Dir()}, f.extraExtDirs...)
	args := []string{
		"--load-extension=" + strings.Join(extDirs, ","),
	}
	if f.signin != nil {
		args = append(args,
			"--load-signin-profile-test-extension="+f.signin.Dir(),
			"--whitelisted-extension-id="+f.signin.ID())
	} else {
		args = append(args, "--whitelisted-extension-id="+f.user.ID())
	}
	return args
}

// RemoveAll removes files for test extensions. It does not remove files for
// extra extensions.
func (f *Files) RemoveAll() error {
	var firstErr error
	if err := f.user.RemoveAll(); err != nil && firstErr == nil {
		firstErr = err
	}
	if f.signin != nil {
		if err := f.signin.RemoveAll(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
