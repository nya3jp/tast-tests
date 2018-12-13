// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

var expectedFirstAPILevelMap = map[string]string{
	"caroline-arcnext": "25",
	"eve-arcnext":      "25",
	"kevin-arcnext":    "25",
	"scarlet-arcnext":  "25",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BuildProperty,
		Desc:         "Checks important properties such as first_api_level",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func BuildProperty(ctx context.Context, s *testing.State) {
	const (
		propertyBoard         = "ro.product.board"
		propertyFirstAPILevel = "ro.product.first_api_level"
		propertySDKVersion    = "ro.build.version.sdk"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	board := getProperty(ctx, s, a, propertyBoard)

	expectedFirstAPILevel, ok := expectedFirstAPILevelMap[board]
	if !ok {
		expectedFirstAPILevel = getProperty(ctx, s, a, propertySDKVersion)
	}

	firstAPILevel := getProperty(ctx, s, a, propertyFirstAPILevel)
	if firstAPILevel != expectedFirstAPILevel {
		s.Fatalf("%v property is %q; want %q", propertyFirstAPILevel,
			firstAPILevel, expectedFirstAPILevel)
	}
}

func getProperty(ctx context.Context, s *testing.State, a *arc.ARC,
	propertyName string) string {
	out, err := a.Command(ctx, "getprop", propertyName).Output()
	if err != nil {
		s.Fatalf("Failed to get %q: %v", propertyName, err)
	}
	return strings.TrimSpace(string(out))
}
