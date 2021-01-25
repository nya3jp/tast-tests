// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
)

// Files manages local files of extensions to be installed to Chrome for
// testing.
type Files struct {
	user         *testExtension
	signin       *testExtension
	extraExtDirs []string
}

// PrepareExtensions writes test extensions to the local disk.
// destDir is a path to a directory under which extensions are written. The
// directory should not exist at the beginning. Callers are responsible for
// deleting the directory after they're done with it.
// The user test extension is always created. If signinExtensionKey is a
// non-empty string, the sign-in profile test extension is also created using
// the key. extraExtDirs specifies directories of extra extensions to be
// installed.
func PrepareExtensions(destDir string, extraExtDirs []string, signinExtensionKey string) (files *Files, retErr error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(destDir)
		}
	}()

	// Prepare the user test extension.
	user, err := prepareTestExtension(filepath.Join(destDir, "test_api"), testExtensionKey, TestExtensionID)
	if err != nil {
		return nil, err
	}

	// Prepare the sign-in profile test extension if it is available.
	var signin *testExtension
	if signinExtensionKey != "" {
		signin, err = prepareTestExtension(filepath.Join(destDir, "test_api_signin_profile"), signinExtensionKey, SigninProfileTestExtensionID)
		if err != nil {
			return nil, err
		}
	}

	// Prepare extra extensions.
	var copiedExtraExtDirs []string
	for i, src := range extraExtDirs {
		manifest := filepath.Join(src, "manifest.json")
		if _, err = os.Stat(manifest); err != nil {
			return nil, errors.Wrap(err, "missing extension manifest")
		}
		dst := filepath.Join(destDir, fmt.Sprintf("extra.%d", i))
		if err := copyDir(src, dst); err != nil {
			return nil, err
		}
		if err := ChownContentsToChrome(dst); err != nil {
			return nil, err
		}
		copiedExtraExtDirs = append(copiedExtraExtDirs, dst)
	}

	return &Files{
		user:         user,
		signin:       signin,
		extraExtDirs: copiedExtraExtDirs,
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

// copyDir copies a directory recursively.
func copyDir(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		src := filepath.Join(srcDir, rel)
		dst := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.Mkdir(dst, 0755)
		}
		return fsutil.CopyFile(src, dst)
	})
}
