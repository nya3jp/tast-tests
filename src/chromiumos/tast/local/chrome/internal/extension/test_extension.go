// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package extension

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

const (
	// TestExtensionID is an extension ID of the autotest extension. It
	// corresponds to testExtensionKey.
	TestExtensionID = "behllobkkfkfnphdnhnkndlbkcpglgmj"

	// SigninProfileTestExtensionID is an id of the test extension which is
	// allowed for signin profile (see http://crrev.com/772709 for details).
	// It corresponds to Var("ui.signinProfileTestExtensionManifestKey").
	SigninProfileTestExtensionID = "mecfefiddjlmabpeilblgegnbioikfmp"

	// testExtensionKey is a manifest key of the autotest extension.
	testExtensionKey = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDuUZGKCDbff6IRaxa4Pue7PPkxwPaNhGT3JEqppEsNWFjM80imEdqMbf3lrWqEfaHgaNku7nlpwPO1mu3/4Hr+XdNa5MhfnOnuPee4hyTLwOs3Vzz81wpbdzUxZSi2OmqMyI5oTaBYICfNHLwcuc65N5dbt6WKGeKgTpp4v7j7zwIDAQAB"
)

// testExtension holds information for a test extension.
type testExtension struct {
	dir string
	id  string
}

// prepareTestExtension prepares a test extension. key is a private key for the
// extension. id is an expected ID of the extension.
func prepareTestExtension(key, id string) (ext *testExtension, retErr error) {
	dir, err := ioutil.TempDir("", "tast_test_api_extension.")
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(dir)
		}
	}()

	actualID, err := writeTestExtension(dir, key)
	if err != nil {
		return nil, err
	}
	if actualID != id {
		return nil, errors.Errorf("unexpected extension ID: got %q; want %q", actualID, id)
	}

	// Chrome hangs with a nonsensical "Extension error: Failed to load extension
	// from: . Manifest file is missing or unreadable." error if an extension directory
	// is owned by another user.
	if err := ChownContentsToChrome(dir); err != nil {
		return nil, err
	}
	return &testExtension{
		dir: dir,
		id:  id,
	}, nil
}

// ID returns the ID of the extension.
func (e *testExtension) ID() string {
	return e.id
}

// Dir returns a directory path where the extension is located.
func (e *testExtension) Dir() string {
	return e.dir
}

// RemoveAll removes files for the test extension.
func (e *testExtension) RemoveAll() error {
	return os.RemoveAll(e.dir)
}

// writeTestExtension writes an empty extension with access to different
// Chrome APIs, needed for performing various tasks without interacting with the
// UI (e.g. enabling the ARC Play Store). Passed key is used for the manifest
// key. The extension's ID is returned.
func writeTestExtension(dir, key string) (id string, err error) {
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// Based on Autotest's client/common_lib/cros/autotest_private_ext/manifest.json and
	// client/cros/multimedia/multimedia_test_extension/manifest.json. Key must be
	// present in the manifest to generate stable extension id.
	var manifest = fmt.Sprintf(`{
  "key": %q,
  "description": "Permits access to various APIs by tests",
  "name": "Test API extension",
  "background": { "scripts": ["background.js"] },
  "incognito": "split",
  "manifest_version": 2,
  "version": "0.1",
  "permissions": [
    "accessibilityFeatures.modify",
    "accessibilityFeatures.read",
    "audio",
    "autotestPrivate",
    "bluetoothPrivate",
    "browsingData",
    "clipboardRead",
    "clipboardWrite",
    "feedbackPrivate",
    "fontSettings",
    "i18n",
    "inputMethodPrivate",
    "languageSettingsPrivate",
    "management",
    "metricsPrivate",
    "notifications",
    "processes",
    "proxy",
    "settingsPrivate",
    "system.display",
    "tabs",
    "wallpaper"
  ],
  "automation": {
    "interact": true,
    "desktop": true
  }
}`, key)

	for _, f := range []struct{ name, data string }{
		{"manifest.json", manifest},
		// Use tast library by default in Test extension.
		{"background.js", tastLibrary},
	} {
		if err = ioutil.WriteFile(filepath.Join(dir, f.name), []byte(f.data), 0644); err != nil {
			return "", err
		}
	}
	id, err = ComputeExtensionID(dir)
	if err != nil {
		return "", err
	}
	return id, nil
}

const (
	// tastLibrary defines the utility library for Tast tests in JavaScript.
	// tast.promisify:
	//   it takes Chrome style async API, which satisfies:
	//   - The last param is a completion callback.
	//   - The completion callback may take an argument, which will be
	//     the result value.
	//   - API error is reported via chrome.runtime.lastError.
	//   Returned value is an async function to call the API.
	// tast.bind:
	//   It takes two arguments: an object, and the name of its method,
	//   then returns a closure that is bound to the given object.
	//   Background: Some Chrome APIs are tied to a JavaScript object, but they
	//   may not be bound to the object. Thus, e.g.
	//
	//     tast.promisify(chrome.accessibilityFeatures.spokenFeedback.set)
	//
	//   returns a Promise instance, which do not call the function on the
	//   expected context. tast.bind can help the situation:
	//
	//     tast.promisify(tast.bind(chrome.accessibilityFeatures.spokenFeedback, "set"))
	tastLibrary = `
tast = {};
tast.promisify = function(f) {
  return (...args) => new Promise((resolve, reject) => {
    f(...args, (val) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
        return;
      }
      resolve(val);
    });
  });
};
tast.bind = function(obj, name) {
  return obj[name].bind(obj);
};
`
)
