// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package allowlist

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
)

type allowlist struct {
	// Hostnames required by Chrome OS for login, enrollemnt and system services.
	Chromeos []string
	// Hostnames required to install Chrome extensions and apps from the Chrome Web Store.
	Extension []string
	// Hostnames required to install Android apps from the Google Play Store.
	Android []string
}

// ReadHostnames reads the hostnames from `path` and returns them. If `arc` is true, it will also
// return hostnames required by the PlayStore. If `ext` is true, it will add to the list hostnames
// required to install extensions.
func ReadHostnames(ctx context.Context, path string, arc, ext bool) ([]string, error) {
	j, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read hostnames file")
	}

	var a allowlist
	if err := json.Unmarshal([]byte(j), &a); err != nil {
		return nil, errors.Wrap(err, "failed to decode the json file")
	}
	hosts := a.Chromeos

	if ext && a.Extension != nil {
		hosts = append(hosts, a.Extension...)
	}
	if arc && a.Android != nil {
		hosts = append(hosts, a.Android...)
	}

	return hosts, nil
}
