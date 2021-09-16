// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeGraphicsInfo,
		Desc: "Check that we can probe cros_healthd for graphics info",
		Contacts: []string{
			"kerker@google.com",
			"cros-tdm@google.com",
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeGraphicsInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryGraphics}
	var graphics graphicsInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &graphics); err != nil {
		s.Fatal("Failed to get graphics telemetry info: ", err)
	}

	if err := verifyGraphicsData(graphics); err != nil {
		s.Fatalf("Failed to validate graphics data, err [%v]", err)
	}
}

type graphicsInfo struct {
	GLESInfo glesInfo `json:"gles_info"`
	EGLInfo  eglInfo  `json:"egl_info"`
}

type glesInfo struct {
	Version        string   `json:"version"`
	ShadingVersion string   `json:"shading_version"`
	Vendor         string   `json:"vendor"`
	Renderer       string   `json:"renderer"`
	Extensions     []string `json:"extensions"`
}

type eglInfo struct {
	Version    string   `json:"version"`
	Vendor     string   `json:"vendor"`
	ClientAPI  string   `json:"client_api"`
	Extensions []string `json:"extensions"`
}

func verifyGLESInfo(gles glesInfo) error {
	if gles.Version == "" {
		return errors.New("Failed. gles.Version is empty")
	}
	if gles.ShadingVersion == "" {
		return errors.New("Failed. gles.ShadingVersion is empty")
	}
	if gles.Vendor == "" {
		return errors.New("Failed. gles.Vendor is empty")
	}
	if gles.Renderer == "" {
		return errors.New("Failed. gles.Renderer is empty")
	}

	return nil
}

func verifyEGLInfo(egl eglInfo) error {
	if egl.Version == "" {
		return errors.New("Failed. egl.Version is empty")
	}
	if egl.Vendor == "" {
		return errors.New("Failed. egl.Vendor is empty")
	}
	if egl.ClientAPI == "" {
		return errors.New("Failed. egl.ClientAPI is empty")
	}

	return nil
}

func verifyGraphicsData(graphics graphicsInfo) error {
	if err := verifyGLESInfo(graphics.GLESInfo); err != nil {
		return err
	}

	if err := verifyEGLInfo(graphics.EGLInfo); err != nil {
		return err
	}

	return nil
}
