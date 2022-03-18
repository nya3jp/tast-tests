// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SocProperties,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks SOC model-related properties (ro.soc.*)",
		Contacts:     []string{"niwa@chromium.org", "matvore@google.com", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,

		// TODO(b/175610620): Merge with BuildProperties once all SOCs
		// can be detected, which will make this testcase
		// non-informational and CQ-blocking.
		Attr: []string{"group:mainline", "informational"},
	})
}

func SocProperties(ctx context.Context, s *testing.State) {
	const (
		propertyManufacturer = "ro.soc.manufacturer"
		propertyModel        = "ro.soc.model"
	)

	a := s.FixtValue().(*arc.PreData).ARC

	// TODO(b/175610620): De-duplicate with the identical anonymous function
	// in BuildProperties, once these two tests are merged. It is tricky to
	// do now because it captures three local vars and args.
	getProperty := func(propertyName string) string {
		var value string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			out, err := a.Command(ctx, "getprop", propertyName).Output()
			if err != nil {
				return err
			}
			value = strings.TrimSpace(string(out))
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatalf("Failed to get %q: %v", propertyName, err)
		}
		return value
	}

	manufacturer := getProperty(propertyManufacturer)
	if re := regexp.MustCompile(`^[0-9A-Za-z ]+$`); !re.MatchString(manufacturer) {
		s.Errorf("%v property is missing or ill-formed: %q", propertyManufacturer, manufacturer)
	}

	model := getProperty(propertyModel)
	if re := regexp.MustCompile(`^[0-9A-Za-z ._/+-]+$`); !re.MatchString(model) {
		s.Errorf("%v property is missing or ill-formed: %q", propertyModel, model)
	}
}
