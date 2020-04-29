// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
)

const (
	// TestExtensionID is an extension ID for the test extension.
	TestExtensionID = "behllobkkfkfnphdnhnkndlbkcpglgmj"

	// SigninProfileTestExtensionID is an id of the test extension which is
	// allowed for signin profile.
	SigninProfileTestExtensionID = "mecfefiddjlmabpeilblgegnbioikfmp"

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

// readKeyFromExtensionManifest returns the decoded public key from an
// extension manifest located at path. An error is returned if the manifest
// is missing or malformed. A nil key is returned if the manifest is
// parsable but doesn't contain a key.
func readKeyFromExtensionManifest(path string) ([]byte, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	j := make(map[string]interface{})
	if err = json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	if enc, ok := j["key"].(string); ok {
		return base64.StdEncoding.DecodeString(enc)
	}
	return nil, nil
}

// ComputeExtensionID computes the 32-character ID that Chrome will use for an unpacked
// extension in dir. If the extension's manifest file contains a public key, it is hashed
// into the ID; otherwise the directory name is hashed.
func ComputeExtensionID(dir string) (string, error) {
	key := []byte(dir)
	mp := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(mp); !os.IsNotExist(err) {
		if k, err := readKeyFromExtensionManifest(mp); err != nil {
			return "", err
		} else if k != nil {
			key = k
		}
	}

	// Chrome computes an extension's ID by creating a SHA-256 digest of the extension's public key
	// and converting its first 16 bytes to 32 hex characters, with the added twist that the
	// characters 'a'-'p' are used rather than '0'-'f'.
	sum := sha256.Sum256(key)
	id := make([]byte, 32)
	for i, b := range sum[:len(id)/2] {
		id[i*2] = b/16 + 'a'
		id[i*2+1] = b%16 + 'a'
	}
	return string(id), nil
}

// writeTestExtension writes an empty extension with autotest_private_ext key.
func writeTestExtension(dir string) (id string, err error) {
	return writeTestExtensionKey(dir, "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDuUZGKCDbff6IRaxa4Pue7PPkxwPaNhGT3JEqppEsNWFjM80imEdqMbf3lrWqEfaHgaNku7nlpwPO1mu3/4Hr+XdNa5MhfnOnuPee4hyTLwOs3Vzz81wpbdzUxZSi2OmqMyI5oTaBYICfNHLwcuc65N5dbt6WKGeKgTpp4v7j7zwIDAQAB")
}

// writeTestExtensionKey writes an empty extension with access to different
// Chrome APIs, needed for performing various tasks without interacting with the
// UI (e.g. enabling the ARC Play Store). Passed key is used for the manifest
// key. The extension's ID is returned.
func writeTestExtensionKey(dir, key string) (id string, err error) {
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// Based on Autotest's client/common_lib/cros/autotest_private_ext/manifest.json and
	// client/cros/multimedia/multimedia_test_extension/manifest.json. Key must be
	// present in the manifest to generate stable extension id.
	const manifestFmt = `{
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
    "clipboardRead",
    "clipboardWrite",
    "fontSettings",
    "i18n",
    "inputMethodPrivate",
    "languageSettingsPrivate",
    "management",
    "metricsPrivate",
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
}`

	for _, f := range []struct{ name, data string }{
		{"manifest.json", fmt.Sprintf(manifestFmt, key)},
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
	if id != TestExtensionID && id != SigninProfileTestExtensionID {
		return "", errors.Errorf("unexpected extension ID: got %q; want %q or %q", id, TestExtensionID, SigninProfileTestExtensionID)
	}
	return id, nil
}

// AddTastLibrary introduces tast library into the page for the given conn.
// This introduces a variable named "tast" to its scope, and it is the
// caller's responsibility to avoid the conflict.
func AddTastLibrary(ctx context.Context, conn *Conn) error {
	return conn.Eval(ctx, tastLibrary, nil)
}
