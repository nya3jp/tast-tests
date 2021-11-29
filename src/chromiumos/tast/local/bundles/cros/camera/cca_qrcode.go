// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAQRCode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the BarcodeDetector API used in CCA",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"cca_qrcode.html", "cca_qrcode.js", "qrcode_3024x3024.jpg"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAQRCode(ctx context.Context, s *testing.State) {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, server.URL+"/cca_qrcode.html")
	if err != nil {
		s.Fatal("Failed to open testing page: ", err)
	}
	defer conn.Close()

	// Name the arguments to make the function call more clear.
	imageURL := "qrcode_3024x3024.jpg"
	expectedCode := "https://www.google.com/chromebook/chrome-os/"
	width := 720
	height := 720
	warmupTimes := 0
	times := 1

	code := fmt.Sprintf("Tast.scan(%q, %d, %d, %q, %d, %d)",
		imageURL, width, height, expectedCode, warmupTimes, times)
	if err := conn.Eval(ctx, code, nil); err != nil {
		s.Error("Failed to scan: ", err)
	}
}
