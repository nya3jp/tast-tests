// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"fmt"
	"os"
	gotesting "testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/ui"
)

func TestToOptions(t *gotesting.T) {
	req := &ui.NewChromeLoginRequest{
		Username: "ABC",
		Credentials: &ui.NewChromeLoginRequest_Credentials{
			Username: "abc",
			Password: "def",
		},
		LoginMode:      ui.NewChromeLoginRequest_LOGIN_MODE_GAIA_LOGIN,
		EnableFeatures: []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
		ExtraArgs:      []string{"--nearby-share-verbose-logging", "--enable-logging"},
	}

	fmt.Fprintf(os.Stderr, "IN test number of foo: %d", 1)

	got, err := toOptions(req)
	var want []chrome.Option

	if err != nil {
		t.Errorf("Failed running toOptions")
	}

	if cmp.Equal(got, want) {
		t.Errorf("Unexpected pixel format, got %v, want %v", got, want)
	}
}
