// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/internal/config"
)

// TastChromeOptioionsJSVar the JavaScript var name for storing the chrome options.
const TastChromeOptioionsJSVar = "tastChromeOptioions"

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
// cfg is the chrome configuration that will be used by the chrome session.
// The user test extension is always created. If SigninExtKey of cfg is a
// non-empty string, the sign-in profile test extension is also created using
// the key. ExtraExtDirs of cfg specifies directories of extra extensions to be
// installed.
// cfg will further be stored into test extension's background.js. It can be
// retrieved later for session reuse comparision.
func PrepareExtensions(destDir string, cfg *config.Config) (files *Files, retErr error) {
	// Ensure destDir does not exist at the beginning.
	if _, err := os.Stat(destDir); err == nil {
		return nil, errors.Errorf("%s must not exist at the beginning", destDir)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(destDir)
		}
	}()

	js, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	extraBgJs := fmt.Sprintf("%s = %q;", TastChromeOptioionsJSVar, js)
	// Prepare the user test extension.
	user, err := prepareTestExtension(filepath.Join(destDir, "test_api"), testExtensionKey, TestExtensionID, extraBgJs)
	if err != nil {
		return nil, err
	}

	// Prepare the sign-in profile test extension if it is available.
	var signin *testExtension
	if cfg.SigninExtKey != "" {
		signin, err = prepareTestExtension(filepath.Join(destDir, "test_api_signin_profile"), cfg.SigninExtKey, SigninProfileTestExtensionID, extraBgJs)
		if err != nil {
			return nil, err
		}
	}

	// Prepare extra extensions.
	var copiedExtraExtDirs []string
	for i, src := range cfg.ExtraExtDirs {
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

// Checksums returns the MD5 checksums of the exsiting extensions' manifest file.
func Checksums(destDir string) ([]string, error) {
	dirs, err := ioutil.ReadDir(destDir)
	if err != nil {
		return nil, err
	}

	var checksums []string
	for _, subdir := range dirs {
		manifest := filepath.Join(destDir, subdir.Name(), "manifest.json")
		manifestContent, err := ioutil.ReadFile(manifest)
		if err != nil {
			// Skip this directory.
			continue
		}
		checksum := md5.Sum(manifestContent)
		checksums = append(checksums, string(checksum[:]))
	}
	return checksums, nil
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
