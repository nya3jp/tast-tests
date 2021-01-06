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
// destDir is a directory under which extensions are written. Callers are
// responsible for deleting destDir after they're done with it. This function
// may save some files under destDir even if it returns an error.
// The user test extension is always created. If signinExtensionKey is a
// non-empty string, the sign-in profile test extension is also created using
// the key. extraExtDirs specifies directories of extra extensions to be
// installed.
// extraBgJs is the extra JavaScript that will be appended to the test extension's
// background.js file.
func PrepareExtensions(destDir string, extraExtDirs []string, signinExtensionKey, extraBgJS string) (*Files, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	// Prepare the user test extension.
	user, err := prepareTestExtension(filepath.Join(destDir, "test_api"), testExtensionKey, TestExtensionID, extraBgJS)
	if err != nil {
		return nil, err
	}

	// Prepare the sign-in profile test extension if it is available.
	var signin *testExtension
	if signinExtensionKey != "" {
		signin, err = prepareTestExtension(filepath.Join(destDir, "test_api_signin_profile"), signinExtensionKey, SigninProfileTestExtensionID, "")
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

// CheckExtensions checks if Chrome extensions are present.
func CheckExtensions(destDir string, extraExtDirs []string, signinExtensionKey string) (*Files, error) {
	var extDirs []string
	var copiedExtraExtDirs []string

	userDir := filepath.Join(destDir, "test_api")
	signinDir := filepath.Join(destDir, "test_api_signin_profile")

	extDirs = append(extDirs, userDir)
	if signinExtensionKey != "" {
		extDirs = append(extDirs, signinDir)
	}
	for i := range extraExtDirs {
		dst := filepath.Join(destDir, fmt.Sprintf("extra.%d", i))
		extDirs = append(extDirs, dst)
		copiedExtraExtDirs = append(copiedExtraExtDirs, dst)
	}

	for _, dir := range extDirs {
		manifest := filepath.Join(dir, "manifest.json")
		if _, err := os.Stat(manifest); err != nil {
			return nil, errors.Wrapf(err, "missing extension manifest for %s", dir)
		}
	}

	userID, err := ComputeExtensionID(userDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute test api extension ID")
	}
	if userID != TestExtensionID {
		return nil, errors.New("unexpected test api extension ID")
	}
	user := &testExtension{dir: userDir, id: userID}

	var signin *testExtension
	if signinExtensionKey != "" {
		signinID, err := ComputeExtensionID(signinDir)
		if err != nil {
			return nil, errors.Wrap(err, "failed to compute signin extension ID")
		}
		if signinID != SigninProfileTestExtensionID {
			return nil, errors.New("unexpected signin extension ID")
		}
		signin = &testExtension{dir: signinID, id: signinID}
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
