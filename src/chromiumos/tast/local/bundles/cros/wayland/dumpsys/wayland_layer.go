// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dumpsys

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

type pointer uint64

type point struct {
	X, Y int
}

func (p *point) Equals(other *point) bool {
	return p.X == other.X && p.Y == other.Y
}

func (p *point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

type rect struct {
	Left, Top     int
	Right, Bottom int
}

type floatRect struct {
	Left, Top     float32
	Right, Bottom float32
}

type waylandLayer struct {
	Address pointer

	MarkedForDeletion   bool
	InvalidBufferFormat bool
	InvalidDataspace    bool
	InvalidTransform    bool
	InvalidBlendMode    bool

	ZOrder      uint32
	Visible     bool
	Hidden      bool
	Alpha       float32
	Buffer      pointer
	ColorBuffer pointer
	Transform   int32

	DisplayFrameScale  float32
	DisplayFrameOffset point
	DisplayFrame       rect
	SourceCrop         floatRect

	InputEnabled     bool
	Resizable        bool
	IMEBlocked       bool
	RootWindow       bool
	HasAndroidShadow bool

	WindowID          int32
	WindowType        int32
	TaskID            int32
	NotificationID    string
	WindowFrame       rect
	WindowFrameScale  float32
	WindowFrameOffset point
	ScaledWindowFrame rect

	StylusTool  bool
	InputRegion string
	Snapshot    bool
}

type parser func(string, *waylandLayer) (string, error)

func makeFlagParser(name string, setter func(*waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		if strings.HasPrefix(content, name) {
			setter(layer)
			content = content[len(name)+1:]
		}
		return content, nil
	}
}

func makeBoolParser(name string, setter func(bool, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+(\d) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: (0|1) ...\", got: %q", name, content)
		}
		value, err := strconv.ParseBool(match[1])
		if err != nil {
			return content, errors.Errorf("invalid value, expected (0|1), got: %q", match[1])
		}
		setter(value, layer)
		return content[len(match[0]):], err
	}
}

func makeInt32Parser(name string, setter func(int32, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+([-+]?\d+) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: int32 ...\", got: %q", name, content)
		}
		value, err := strconv.ParseInt(match[1], 10, 32)
		if err != nil {
			return content, errors.Errorf("invalid value, expected int32, got: %q", match[1])
		}
		setter(int32(value), layer)
		return content[len(match[0]):], err
	}
}

func makeUint32Parser(name string, setter func(uint32, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+(\d+) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: uint32 ...\", got: %q", name, content)
		}
		value, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil {
			return content, errors.Errorf("invalid value, expected uint32, got: %q", match[1])
		}
		setter(uint32(value), layer)
		return content[len(match[0]):], err
	}
}

func makeFloat32Parser(name string, setter func(float32, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+([-+]?\d*\.?\d+) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: float32 ...\", got: %q", name, content)
		}
		value, err := strconv.ParseFloat(match[1], 32)
		if err != nil {
			return content, errors.Errorf("invalid value, expected float32, got: %q", match[1])
		}
		setter(float32(value), layer)
		return content[len(match[0]):], err
	}
}

func makePointerParser(name string, setter func(pointer, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+0x([0-9a-f]+) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: float32 ...\", got: %q", name, content)
		}
		value, err := strconv.ParseUint(match[1], 16, 64)
		if err != nil {
			return content, errors.Errorf("invalid value, expected float32, got: %q", match[1])
		}
		setter(pointer(value), layer)
		return content[len(match[0]):], err
	}
}

func makePointParser(name string, setter func(point, *waylandLayer)) parser {
	return func(content string, layer *waylandLayer) (string, error) {
		re := regexp.MustCompile(`^` + name + `:\s+([-+]?\d+) ([-+]?\d+) `)
		match := re.FindStringSubmatch(content)
		if match == nil {
			return content, errors.Errorf("invalid format, expected \"%s: int32 ...\", got: %q", name, content)
		}
		x, err := strconv.ParseInt(match[1], 10, 32)
		if err != nil {
			return content, errors.Errorf("invalid x value, expected int32, got: %q", match[1])
		}
		y, err := strconv.ParseInt(match[2], 10, 32)
		if err != nil {
			return content, errors.Errorf("invalid y value, expected int32, got: %q", match[2])
		}
		setter(point{X: int(x), Y: int(y)}, layer)
		return content[len(match[0]):], err
	}
}

func parseLayer(content string, layer *waylandLayer) (string, error) {
	re := regexp.MustCompile(`^Layer 0x([0-9a-f]{8}) `)
	match := re.FindStringSubmatch(content)
	if match == nil {
		return content, errors.Errorf("invalid format, expected: \"Layer ${ADDR} ...\", got: %q", content)
	}
	address, err := strconv.ParseUint(match[1], 16, 64)
	if err != nil {
		return content, errors.Errorf("invalid layer address, expected: int, got: %q", match[1])
	}
	layer.Address = pointer(address)
	return content[len(match[0]):], err
}

var parseMarkedForDeletion = makeFlagParser("(marked for deletion!)", func(layer *waylandLayer) {
	layer.MarkedForDeletion = true
})

var parseInvalidBufferFormat = makeFlagParser("(inv fmt!)", func(layer *waylandLayer) {
	layer.InvalidBufferFormat = true
})

var parseInvalidDataspace = makeFlagParser("(inv datasp!)", func(layer *waylandLayer) {
	layer.InvalidDataspace = true
})

var parseInvalidTransform = makeFlagParser("(inv xform!)", func(layer *waylandLayer) {
	layer.InvalidTransform = true
})

var parseInvalidBlendMode = makeFlagParser("(inv blend!)", func(layer *waylandLayer) {
	layer.InvalidBlendMode = true
})

var parseZOrder = makeUint32Parser("Z", func(value uint32, layer *waylandLayer) {
	layer.ZOrder = value
})

var parseVisible = makeBoolParser("visible", func(value bool, layer *waylandLayer) {
	layer.Visible = value
})

var parseHidden = makeBoolParser("hidden", func(value bool, layer *waylandLayer) {
	layer.Hidden = value
})

var parseAlpha = makeFloat32Parser("alpha", func(value float32, layer *waylandLayer) {
	layer.Alpha = value
})

var parseBuffer = makePointerParser("gralloc buffer", func(value pointer, layer *waylandLayer) {
	layer.Buffer = value
})

var parseColorBuffer = makePointerParser("color buffer", func(value pointer, layer *waylandLayer) {
	layer.ColorBuffer = value
})

var parseTransform = makeInt32Parser("transform", func(value int32, layer *waylandLayer) {
	layer.Transform = value
})

var parseDisplayFrameScale = makeFloat32Parser("display frame scale", func(value float32, layer *waylandLayer) {
	layer.DisplayFrameScale = value
})

var parseDisplayFrameOffset = makePointParser("display frame offset", func(value point, layer *waylandLayer) {
	layer.DisplayFrameOffset = value
})
