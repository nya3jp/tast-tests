// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

const (
	// Path to ARC features file on the device.
	deviceArcFeatures = "/etc/arc/features.json"

	// Path to ARC ureadahead pack on the device.
	deviceArcUreadaheadPack = "/var/lib/ureadahead/opt.google.containers.android.rootfs.root.pack"

	// Base path for uploaded ureadahead packs.
	serverUreadaheadPackRoot = "gs://chromeos-arc-images/ureadahead_packs"

	// Name of the pack in case of initial boot.
	initialPack = "initial_pack"

	// Name of the pack in case of provisioned boot.
	provisionedPack = "provisioned_pack"
)

// features contains minimalist definition to parse required information.
type features struct {
	Properties properties `json:"properties"`
}

// properties contains minimalist definition to build version and abi.
type properties struct {
	Build string `json:"ro.build.version.incremental"`
	Abi   string `json:"ro.product.cpu.abi"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DataCollector,
		Desc: "Signs in to DUT and performs ARC++ boot with various paramters. Capture required data and upload it to Chrome binary server. This data is used by various tools. Normally, this test should be run during the Android PFQ, once per build/arch",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		// For the initial period, mark it informational.
		Attr:         []string{"group:informational"},
		SoftwareDeps: []string{"android", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.UreadaheadPackService"},
		Timeout:      5 * time.Minute,
	})
}

// getArcVersion gets arc features json file from the devices, parses for build id, abi, and return
// this information as a combined string.
func getArcVersion(ctx context.Context, s *testing.State, dut *dut.DUT) (string, error) {
	featuresPath := filepath.Join(s.OutDir(), "arc_features")
	if err := dut.GetFile(ctx, deviceArcFeatures, featuresPath); err != nil {
		return "", errors.Wrap(err, "failed to get ARC features from the device")
	}

	jsonFile, err := os.Open(featuresPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open ARC features")
	}
	defer jsonFile.Close()

	b, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read ARC features")
	}

	var f features
	if err = json.Unmarshal([]byte(b), &f); err != nil {
		return "", errors.Wrap(err, "failed to parse ARC features")
	}

	result := fmt.Sprintf("%s_%s", f.Properties.Abi, f.Properties.Build)
	return result, nil
}

// uploadUreadaheadPack gets ureadahead pack from the targed device and uploads it to the server.
func uploadUreadaheadPack(ctx context.Context, s *testing.State, dut *dut.DUT, version string, dst string) error {
	packPath := filepath.Join(s.OutDir(), dst)
	if err := dut.GetFile(ctx, deviceArcUreadaheadPack, packPath); err != nil {
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
	// first generation might be incomplete. Just pass it.
	if _, err := service.Generate(ctx, &request); err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for warm-up pass: ", err)
	}

	if _, err := service.Generate(ctx, &request); err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for initial boot pass: ", err)
	}

	if err = uploadUreadaheadPack(ctx, s, d, v, initialPack); err != nil {
		s.Fatal("Failed to upload initial boot pack: ", err)
	}

	request.InitialBoot = false
	if _, err := service.Generate(ctx, &request); err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for second boot pass: ", err)
	}

	if err = uploadUreadaheadPack(ctx, s, d, v, provisionedPack); err != nil {
		s.Fatal("Failed to upload provisioned boot pack: ", err)
	}
}
