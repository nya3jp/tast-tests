// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

# Handwriting Input Testing
Handwriting.go contains functions used to read SVG files, generate and populate strokes, and draw the SVG file on the handwriting input.

## Creating the handwriting input test SVG file
Any SVG editor can be used to create the handwriting input test SVG file, however, this file must be created using LineTo commands only.
There are many online editors that specifically use LineTo commands only, or the Line tool can be used to create the file.

## Detailed implementation documentation
See go/tast-handwriting-svg-parsing