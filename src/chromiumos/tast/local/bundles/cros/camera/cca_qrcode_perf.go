// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	mediacpu "chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAQRCodePerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the BarcodeDetector API used in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"cca_qrcode.html", "cca_qrcode.js", "qrcode_3024x3024.jpg"},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

func CCAQRCodePerf(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, server.URL+"/cca_qrcode.html")
	if err != nil {
		s.Fatal("Failed to open testing page: ", err)
	}
	defer conn.Close()

	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark: ", err)
	}
	defer cleanUpBenchmark(ctx)

	// Leave a bit of time to clean up things.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU become idle")
	}

	pv := perf.NewValues()

	// Name the arguments to make the function call more clear.
	imageURL := "qrcode_3024x3024.jpg"
	expectedCode := "https://www.google.com/chromebook/chrome-os/"
	resolutions := []struct {
		Width  int
		Height int
	}{{1280, 720}, {720, 720}, {640, 480}, {480, 480}}
	warmupTimes := 3
	times := 30

	for _, res := range resolutions {
		code := fmt.Sprintf("Tast.scan(%q, %d, %d, %q, %d, %d)",
			imageURL, res.Width, res.Height, expectedCode, warmupTimes, times)
		avgTime := 0.0
		if err := conn.Eval(ctx, code, &avgTime); err != nil {
			s.Errorf("Failed to scan %dx%d: %v", res.Width, res.Height, err)
			continue
		}
		name := fmt.Sprintf("qrcode_%dx%d", res.Width, res.Height)
		s.Logf("%s: %.1fms", name, avgTime)
		pv.Set(perf.Metric{
			Name:      name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, avgTime)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
