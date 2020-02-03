// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

const (
	// Base path for uploaded ureadahead packs.
	serverUreadaheadPackRoot = "gs://chromeos-arc-images/ureadahead_packs"

	// Name of the pack in case of initial boot.
	initialPack = "initial_pack"

	// Name of the pack in case of provisioned boot.
	provisionedPack = "provisioned_pack"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataCollector,
		Desc: "Signs in to DUT and performs ARC++ boot with various paramters. Capture required data and upload it to Chrome binary server. This data is used by various tools. Normally, this test should be run during the Android PFQ, once per build/arch",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		// For the initial period, mark it informational.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.UreadaheadPackService"},
		Timeout:      5 * time.Minute,
	})
}

// getArcVersion gets ARC build properties from the device, parses for build id, abi, and
// returns these fields as a combined string.
func getArcVersion(ctx context.Context, s *testing.State, dut *dut.DUT) (string, error) {
	var propertyFile string

	out, err := dut.Command("cat", "/run/chrome/is_arcvm").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to check ARCVM status remotely")
	}

	if string(out) == "1" {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/run/arc/properties/build.prop"
	}

	out, err = dut.Command("cat", propertyFile).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read ARC build property file remotely")
	}

	mArch := regexp.MustCompile(`(\n|^)ro.product.cpu.abi=(.+)(\n|$)`).FindStringSubmatch(string(out))
	if mArch == nil {
		return "", errors.New("ro.product.cpu.abi is not found")
	}

	// Note, this should work on official builds only. Custom built Android images contains the
	// version in different format.
	mVersion := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(\d+)(\n|$)`).FindStringSubmatch(string(out))
	if mVersion == nil {
		return "", errors.New("Valid ro.build.version.incremental is not found")
	}

	result := fmt.Sprintf("%s_%s", mArch[2], mVersion[2])
	return result, nil
}

// uploadUreadaheadPack gets ureadahead pack from the targed device and uploads it to the server.
func uploadUreadaheadPack(ctx context.Context, s *testing.State, dut *dut.DUT, version string, src string, dst string) error {
	packPath := filepath.Join(s.OutDir(), dst)
	if err := dut.GetFile(ctx, src, packPath); err != nil {
		return errors.Wrap(err, "failed to get ARC ureadahead pack from the device")
	}

	gsURL := fmt.Sprintf("%s/%s/%s", serverUreadaheadPackRoot, version, dst)

	// Use gsutil command to upload the pack to the server.
	cmd := exec.Command("gsutil", "copy", packPath, gsURL)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "failed to upload ARC ureadahead pack to the server")
	}

	s.Logf("Uploaded ARC ureadahead pack to the server: %q", gsURL)
	return nil
}

func DataCollector(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	v, err := getArcVersion(ctx, s, d)
	if err != nil {
		s.Fatal("Failed to get ARC version: ", err)
	}
	s.Logf("Detected version: %s", v)

	service := arc.NewUreadaheadPackServiceClient(cl.Conn)

	var request arcpb.UreadaheadPackRequest
	request.InitialBoot = true

	// Due to race condition of using ureadahead in various parts of Chrome,
	// first generation might be incomplete. Just pass it without analyzing.
	if _, err := service.Generate(ctx, &request); err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for warm-up pass: ", err)
	}

	// Pass initial boot and capture results.
	response, err := service.Generate(ctx, &request)
	if err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for initial boot pass: ", err)
	}

	if err = uploadUreadaheadPack(ctx, s, d, v, response.PackPath, initialPack); err != nil {
		s.Fatal("Failed to upload initial boot pack: ", err)
	}

	// Now pass provisioned boot and capture results.
	request.InitialBoot = false
	response, err = service.Generate(ctx, &request)
	if err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for second boot pass: ", err)
	}

	if err = uploadUreadaheadPack(ctx, s, d, v, response.PackPath, provisionedPack); err != nil {
		s.Fatal("Failed to upload provisioned boot pack: ", err)
	}
}
