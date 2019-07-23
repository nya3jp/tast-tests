// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumpsys

import "testing"

func TestParseLayerCorrect(t *testing.T) {
	content := "Layer 0xe9961000 Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.Address = 0

	after, err := parseLayer(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Address != 0xe9961000 {
		t.Errorf("Invalid address, expected: 0xe9961000, got: 0x%x", layer.Address)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseLayerInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseLayer(content, layer)
	if err == nil {
		t.Fatal("Expected parseLayer to fail")
	}
}

func TestParseLayerInvalidValue(t *testing.T) {
	content := "Layer 0xe99610gg Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)

	_, err := parseLayer(content, layer)
	if err == nil {
		t.Fatal("Expected parseLayer to fail")
	}
}

func TestParseMarkedForDeletionPresent(t *testing.T) {
	content := "(marked for deletion!) Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.MarkedForDeletion = false

	after, err := parseMarkedForDeletion(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.MarkedForDeletion != true {
		t.Error("Invalid value, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseMarkedForDeletionAbsent(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.MarkedForDeletion = false

	after, err := parseMarkedForDeletion(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.MarkedForDeletion != false {
		t.Error("Invalid value, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidBufferFormatPresent(t *testing.T) {
	content := "(inv fmt!) Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidBufferFormat = false

	after, err := parseInvalidBufferFormat(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidBufferFormat != true {
		t.Error("Invalid value, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidBufferFormatAbsent(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidBufferFormat = false

	after, err := parseInvalidBufferFormat(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidBufferFormat != false {
		t.Error("Invalid value, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidDataspacePresent(t *testing.T) {
	content := "(inv datasp!) Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidDataspace = false

	after, err := parseInvalidDataspace(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidDataspace != true {
		t.Error("Invalid value, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidDataspaceAbsent(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidDataspace = false

	after, err := parseInvalidDataspace(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidDataspace != false {
		t.Error("Invalid value, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidTransformPresent(t *testing.T) {
	content := "(inv xform!) Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidTransform = false

	after, err := parseInvalidTransform(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidTransform != true {
		t.Error("Invalid value, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidTransformAbsent(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidTransform = false

	after, err := parseInvalidTransform(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidTransform != false {
		t.Error("Invalid value, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidBlendModePresent(t *testing.T) {
	content := "(inv blend!) Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidBlendMode = false

	after, err := parseInvalidBlendMode(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidBlendMode != true {
		t.Error("Invalid value, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseInvalidBlendModeAbsent(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.InvalidBlendMode = false

	after, err := parseInvalidBlendMode(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.InvalidBlendMode != false {
		t.Error("Invalid value, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseZOrderCorrect(t *testing.T) {
	content := "Z:      20 visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.ZOrder = 0

	after, err := parseZOrder(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.ZOrder != 20 {
		t.Errorf("Invalid z order, expected: 20, got: %d", layer.ZOrder)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseZOrderInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseZOrder(content, layer)
	if err == nil {
		t.Fatal("Expected parseZOrder to fail")
	}
}

func TestParseZOrderInvalidValue(t *testing.T) {
	content := "Z:    0xff visible: 1 hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)

	_, err := parseZOrder(content, layer)
	if err == nil {
		t.Fatal("Expected parseZOrder to fail")
	}
}

func TestParseVisibleCorrect(t *testing.T) {
	content := "visible: 1 hidden: 0 alpha: 1.0"
	contentAfter := "hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)
	layer.Visible = false

	after, err := parseVisible(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Visible != true {
		t.Error("Invalid z order, expected: true, got: false")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseVisibleInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseVisible(content, layer)
	if err == nil {
		t.Fatal("Expected parseVisible to fail")
	}
}

func TestParseVisibleInvalidValue(t *testing.T) {
	content := "visible: yes hidden: 0 alpha: 1.0"
	layer := new(waylandLayer)

	_, err := parseVisible(content, layer)
	if err == nil {
		t.Fatal("Expected parseVisible to fail")
	}
}

func TestParseHiddenCorrect(t *testing.T) {
	content := "hidden: 0 alpha: 1.0"
	contentAfter := "alpha: 1.0"
	layer := new(waylandLayer)
	layer.Hidden = true

	after, err := parseHidden(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Hidden != false {
		t.Error("Invalid z order, expected: false, got: true")
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseHiddenInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseHidden(content, layer)
	if err == nil {
		t.Fatal("Expected parseHidden to fail")
	}
}

func TestParseHiddenInvalidValue(t *testing.T) {
	content := "hidden: no alpha: 1.0"
	layer := new(waylandLayer)

	_, err := parseHidden(content, layer)
	if err == nil {
		t.Fatal("Expected parseHidden to fail")
	}
}

func TestParseAlphaCorrect(t *testing.T) {
	content := "alpha: 1.0 gralloc buffer: 0xe9da0d80 color buffer: 0x0 transform: 0"
	contentAfter := "gralloc buffer: 0xe9da0d80 color buffer: 0x0 transform: 0"
	layer := new(waylandLayer)
	layer.Alpha = 0.0

	after, err := parseAlpha(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Alpha != 1.0 {
		t.Errorf("Invalid z order, expected: 1.0, got: %f", layer.Alpha)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseAlphaInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseAlpha(content, layer)
	if err == nil {
		t.Fatal("Expected parseAlpha to fail")
	}
}

func TestParseAlphaInvalidValue(t *testing.T) {
	content := "alpha: true gralloc buffer: 0xe9da0d80 color buffer: 0x0 transform: 0"
	layer := new(waylandLayer)

	_, err := parseAlpha(content, layer)
	if err == nil {
		t.Fatal("Expected parseAlpha to fail")
	}
}

func TestParseBufferCorrect(t *testing.T) {
	content := "gralloc buffer: 0xe9da0d80 color buffer: 0x0 transform: 0"
	contentAfter := "color buffer: 0x0 transform: 0"
	layer := new(waylandLayer)
	layer.Buffer = 0

	after, err := parseBuffer(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Buffer != 0xe9da0d80 {
		t.Errorf("Invalid z order, expected: 0xe9da0d80, got: 0x%x", layer.Buffer)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseBufferInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseBuffer(content, layer)
	if err == nil {
		t.Fatal("Expected parseBuffer to fail")
	}
}

func TestParseBufferInvalidValue(t *testing.T) {
	content := "gralloc buffer: 0xe9da0dgg color buffer: 0x0 transform: 0"
	layer := new(waylandLayer)

	_, err := parseBuffer(content, layer)
	if err == nil {
		t.Fatal("Expected parseBuffer to fail")
	}
}

func TestParseColorBufferCorrect(t *testing.T) {
	content := "color buffer: 0xe9da0d80 transform: 0"
	contentAfter := "transform: 0"
	layer := new(waylandLayer)
	layer.ColorBuffer = 0

	after, err := parseColorBuffer(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.ColorBuffer != 0xe9da0d80 {
		t.Errorf("Invalid z order, expected: 0xe9da0d80, got: 0x%x", layer.ColorBuffer)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseColorBufferInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseColorBuffer(content, layer)
	if err == nil {
		t.Fatal("Expected parseColorBuffer to fail")
	}
}

func TestParseColorBufferInvalidValue(t *testing.T) {
	content := "color buffer: 0xgg transform: 0"
	layer := new(waylandLayer)

	_, err := parseColorBuffer(content, layer)
	if err == nil {
		t.Fatal("Expected parseColorBuffer to fail")
	}
}

func TestParseTransformCorrect(t *testing.T) {
	content := "transform: 0 display frame scale: 1.00 display frame offset: 0 0"
	contentAfter := "display frame scale: 1.00 display frame offset: 0 0"
	layer := new(waylandLayer)
	layer.Transform = -1

	after, err := parseTransform(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.Transform != 0 {
		t.Errorf("Invalid z order, expected: 0, got: %d", layer.Transform)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseTransformInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseTransform(content, layer)
	if err == nil {
		t.Fatal("Expected parseTransform to fail")
	}
}

func TestParseTransformInvalidValue(t *testing.T) {
	content := "transform: zero display frame scale: 1.00 display frame offset: 0 0"
	layer := new(waylandLayer)

	_, err := parseTransform(content, layer)
	if err == nil {
		t.Fatal("Expected parseTransform to fail")
	}
}

func TestParseDisplayFrameScaleCorrect(t *testing.T) {
	content := "display frame scale: 1.00 display frame offset: 0 0"
	contentAfter := "display frame offset: 0 0"
	layer := new(waylandLayer)
	layer.DisplayFrameScale = 0.0

	after, err := parseDisplayFrameScale(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if layer.DisplayFrameScale != 1.0 {
		t.Errorf("Invalid z order, expected: 1.0, got: %f", layer.DisplayFrameScale)
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseDisplayFrameScaleInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseDisplayFrameScale(content, layer)
	if err == nil {
		t.Fatal("Expected parseDisplayFrameScale to fail")
	}
}

func TestParseDisplayFrameScaleInvalidValue(t *testing.T) {
	content := "display frame scale: one display frame offset: 0 0"
	layer := new(waylandLayer)

	_, err := parseDisplayFrameScale(content, layer)
	if err == nil {
		t.Fatal("Expected parseDisplayFrameScale to fail")
	}
}

func TestParseDisplayFrameOffsetCorrect(t *testing.T) {
	content := "display frame offset: 0 0 display:  1199   743  1200   744"
	contentAfter := "display:  1199   743  1200   744"
	layer := new(waylandLayer)
	layer.DisplayFrameOffset = point{1, 1}

	after, err := parseDisplayFrameOffset(content, layer)
	if err != nil {
		t.Fatal("Unexpected error: ", err)
	}
	if !layer.DisplayFrameOffset.Equals(&point{0, 0}) {
		t.Errorf("Invalid z order, expected: (0, 0), got: %s", layer.DisplayFrameOffset.String())
	}
	if after != contentAfter {
		t.Errorf("Invalid content returned, expected: %q, got: %q", contentAfter, after)
	}
}

func TestParseDisplayFrameOffsetInvalidFormat(t *testing.T) {
	content := "Wayland service state dump"
	layer := new(waylandLayer)
	_, err := parseDisplayFrameOffset(content, layer)
	if err == nil {
		t.Fatal("Expected parseDisplayFrameOffset to fail")
	}
}

func TestParseDisplayFrameOffsetInvalidValue(t *testing.T) {
	content := "display frame offset: 0 display:  1199   743  1200   744"
	layer := new(waylandLayer)

	_, err := parseDisplayFrameOffset(content, layer)
	if err == nil {
		t.Fatal("Expected parseDisplayFrameOffset to fail")
	}
}
