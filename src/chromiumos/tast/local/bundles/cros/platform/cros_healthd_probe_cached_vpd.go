// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"reflect"

	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeCachedVpd,
		Desc: "Check that we can probe cros_healthd for cached vpd info",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"cros_config", "diagnostics"},
	})
}

func CrosHealthdProbeCachedVpd(ctx context.Context, s *testing.State) {
	records, err := croshealthd.RunAndParseTelem(ctx, croshealthd.TelemCategoryCachedVPD, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get cached VPD telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of output lines: got %d; want 2", len(records))
	}

	// Verify the headers.
	want := []string{"sku_number"}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Check if the device has a SKU number. If it does, the SKU number should
	// be printed. If it does not, "NA" should be printed.
	val, err := crosconfig.Get(ctx, "/cros-healthd/cached-vpd", "has-sku-number")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-sku-number property: ", err)
	}

	hasSku := err == nil && val == "true"
	sku := records[1][0]
	if hasSku && sku == "" {
		s.Fatal("Empty SKU number")
	}

	if !hasSku && sku != "NA" {
		s.Fatalf("Incorrect SKU number: got %v, want NA", sku)
	}
}
