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

// Extensions manages a set of extensions installed to Chrome for testing.
type Extensions struct {
	user         *testExtension
	signin       *testExtension
	extraExtDirs []string
}

// PrepareExtensions prepares extensions to be installed to Chrome for testing.
// The test extension is always installed. extraExtDirs specifies a directory
// list of extra extensions be installed.
// If signinExtensionKey is a non-empty string, it also installs the sign-in
// profile test extension using the private key.
func PrepareExtensions(extraExtDirs []string, signinExtensionKey string) (exts *Extensions, retErr error) {
	// Prepare the built-in test extension.
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

	return &Extensions{
		user:         user,
		signin:       signin,
		extraExtDirs: extraExtDirs,
	}, nil
}

// DeprecatedDirs returns a list of directories where extensions are available.
//
// DEPRECATED: Use ChromeArgs instead. This method does not handle sign-in
// profile extensions correctly.
func (e *Extensions) DeprecatedDirs() []string {
	return append([]string{e.user.Dir()}, e.extraExtDirs...)
}

// ChromeArgs returns a list of arguments to be passed to Chrome to enable
// extensions.
func (e *Extensions) ChromeArgs() []string {
	extDirs := append([]string{e.user.Dir()}, e.extraExtDirs...)
	args := []string{
		"--load_extension=" + strings.Join(extDirs, ","),
	}
	if e.signin != nil {
		args = append(args,
			"--load-signin-profile-test-extension="+e.signin.Dir(),
			"--whitelisted-extension-id="+e.signin.ID())
	} else {
		args = append(args, "--whitelisted-extension-id="+e.user.ID())
	}
	return args
}

// RemoveAll removes files for test extensions. It does not remove files for
// extra extensions.
func (e *Extensions) RemoveAll() error {
	var firstErr error
	if err := e.user.RemoveAll(); err != nil && firstErr == nil {
		firstErr = err
	}
	if e.signin != nil {
		if err := e.signin.RemoveAll(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
