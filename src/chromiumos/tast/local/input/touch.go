// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"time"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// TouchCoord describes an X or Y coordinate in touchscreen coordinates
// (rather than pixels).
type TouchCoord int32

// TouchscreenEventWriter supports injecting touch events into a touchscreen device.
// It supports multitouch as defined in "Protocol Example B" here:
//  https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt
//  https://www.kernel.org/doc/Documentation/input/event-codes.txt
// This is partial implementation of the multi-touch specification. Each injected
// touch event contains the following codes:
//  - ABS_MT_TRACKING_ID
//  - ABS_MT_POSITION_X & ABS_X
//  - ABS_MT_POSITION_Y & ABS_Y
//  - ABS_MT_PRESSURE & ABS_PRESSURE
//  - ABS_MT_TOUCH_MAJOR
//  - ABS_MT_TOUCH_MINOR
//  - BTN_TOUCH
// Any other code, like MSC_TIMESTAMP, is not implemented.
type TouchscreenEventWriter struct {
	rw            *RawEventWriter
	virt          *os.File // if non-nil, used to hold a virtual device open
	dev           string   // path to underlying device in /dev/input
	nextTouchID   int32
	width         TouchCoord
	height        TouchCoord
	maxTouchSlot  int
	maxTrackingID int
	maxPressure   int

	// clockwise rotation in degree to translate event location. It only supports
	// 0, 90, 180, or 270 degrees.
	rotation int
}

var nextVirtTouchNum = 1 // appended to virtual touchscreen device name

const touchFrequency = 5 * time.Millisecond

// ZoomType represents the zoom type to perform.
type ZoomType int

// Holds all the zoom types that can be performed.
const (
	ZoomIn ZoomType = iota
	ZoomOut
)

// Touchscreen returns an TouchscreenEventWriter to inject events into an arbitrary touchscreen device.
func Touchscreen(ctx context.Context) (*TouchscreenEventWriter, error) {
	infos, err := readDevices("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		if !info.isTouchscreen() {
			continue
		}
		testing.ContextLogf(ctx, "Opening touchscreen device %+v", info)

		// Get touchscreen properties: bounds, max touches, max pressure and max track id.
		f, err := os.Open(info.path)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var infoX, infoY, infoSlot, infoTrackingID, infoPressure absInfo
		for _, entry := range []struct {
			ec  EventCode
			dst *absInfo
		}{
			{ABS_X, &infoX},
			{ABS_Y, &infoY},
			{ABS_MT_SLOT, &infoSlot},
			{ABS_MT_TRACKING_ID, &infoTrackingID},
			{ABS_MT_PRESSURE, &infoPressure},
		} {
			if err := ioctl(int(f.Fd()), evIOCGAbs(uint(entry.ec)), uintptr(unsafe.Pointer(entry.dst))); err != nil {
				return nil, err
			}
		}

		if infoTrackingID.maximum < infoSlot.maximum {
			return nil, errors.Errorf("invalid MT tracking ID %d; should be >= max slots %d",
				infoTrackingID.maximum, infoSlot.maximum)
		}

		if infoX.maximum == 0 || infoY.maximum == 0 {
			return nil, errors.Errorf("invalid screen size (%d, %d)", infoX.maximum, infoY.maximum)
		}

		device, err := Device(ctx, info.path)
		if err != nil {
			return nil, err
		}
		return &TouchscreenEventWriter{
			rw:            device,
			width:         TouchCoord(infoX.maximum),
			height:        TouchCoord(infoY.maximum),
			maxTouchSlot:  int(infoSlot.maximum),
			maxTrackingID: int(infoTrackingID.maximum),
			maxPressure:   int(infoPressure.maximum),
		}, nil
	}

	// If we didn't find a real touchscreen, create a virtual one.
	return VirtualTouchscreen(ctx)
}

// FindPhysicalTouchscreen iterates over devices and returns path for a physical touchscreen,
// otherwise returns boolean indicating a physical touchscreen was not found.
func FindPhysicalTouchscreen(ctx context.Context) (bool, string, error) {
	infos, err := readDevices("")
	if err != nil {
		return false, "", errors.Wrap(err, "failed to read devices")
	}
	for _, info := range infos {
		if info.isTouchscreen() {
			testing.ContextLogf(ctx, "Using existing touch screen device %+v", info)
			return true, info.path, nil
		}
	}
	return false, "", nil
}

// VirtualTouchscreen creates a virtual touchscreen device and returns an EventWriter that injects events into it.
func VirtualTouchscreen(ctx context.Context) (*TouchscreenEventWriter, error) {
	const (
		// Most touchscreens use I2C bus. But hardcoding to USB since it is supported
		// in all Chromebook devices.
		busType = 0x3 // BUS_USB from input.h

		// Device constants taken from Chromebook Slate.
		vendor  = 0x2d1f
		product = 0x5143
		version = 0x100

		// Input characteristics.
		props   = 1 << INPUT_PROP_DIRECT
		evTypes = 1<<EV_KEY | 1<<EV_ABS | 1<<EV_MSC

		// Abs axes supported in our virtual device.
		absSupportedAxes = 1<<ABS_X | 1<<ABS_Y | 1<<ABS_PRESSURE | 1<<ABS_MT_SLOT |
			1<<ABS_MT_TOUCH_MAJOR | 1<<ABS_MT_TOUCH_MINOR | 1<<ABS_MT_ORIENTATION |
			1<<ABS_MT_POSITION_X | 1<<ABS_MT_POSITION_Y | 1<<ABS_MT_TOOL_TYPE |
			1<<ABS_MT_TRACKING_ID | 1<<ABS_MT_PRESSURE

		// Abs axis constants. Taken from Chromebook Slate.
		axisMaxX            = 10404
		axisMaxY            = 6936
		axisMaxTracking     = 65535
		axisMaxPressure     = 255
		axisCoordResolution = 40
	)
	axisMaxTouchSlot := 9

	// Include our PID in the device name to be extra careful in case an old bundle process hasn't exited.
	name := fmt.Sprintf("Tast virtual touchscreen %d.%d", os.Getpid(), nextVirtTouchNum)
	nextVirtTouchNum++
	testing.ContextLogf(ctx, "Creating virtual touchscreen device %q", name)

	dev, virt, err := createVirtual(name, devID{busType, vendor, product, version}, props, evTypes,
		map[EventType]*big.Int{
			EV_KEY: makeBigInt([]uint64{0x400, 0, 0, 0, 0, 0}), // BTN_TOUCH
			EV_ABS: big.NewInt(absSupportedAxes),
			EV_MSC: big.NewInt(1 << MSC_TIMESTAMP),
		}, map[EventCode]Axis{
			ABS_X:              {axisMaxX, 0, 0, 0, axisCoordResolution},
			ABS_Y:              {axisMaxY, 0, 0, 0, axisCoordResolution},
			ABS_PRESSURE:       {axisMaxPressure, 0, 0, 0, 0},
			ABS_MT_SLOT:        {int32(axisMaxTouchSlot), 0, 0, 0, 0},
			ABS_MT_TOUCH_MAJOR: {255, 0, 0, 0, 1},
			ABS_MT_TOUCH_MINOR: {255, 0, 0, 0, 1},
			ABS_MT_ORIENTATION: {1, 0, 0, 0, 0},
			ABS_MT_POSITION_X:  {axisMaxX, 0, 0, 0, axisCoordResolution},
			ABS_MT_POSITION_Y:  {axisMaxY, 0, 0, 0, axisCoordResolution},
			ABS_MT_TOOL_TYPE:   {2, 0, 0, 0, 0},
			ABS_MT_TRACKING_ID: {axisMaxTracking, 0, 0, 0, 0},
			ABS_MT_PRESSURE:    {axisMaxPressure, 0, 0, 0, 0},
		})
	if err != nil {
		return nil, err
	}

	// After initializing the virtual device a pause is needed to be able to detect the device.
	// TODO(crbug.com/1015264): Remove the hard-coded sleep.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return nil, err
	}

	device, err := Device(ctx, dev)
	if err != nil {
		return nil, err
	}
	return &TouchscreenEventWriter{
		rw:            device,
		dev:           dev,
		virt:          virt,
		width:         axisMaxX,
		height:        axisMaxY,
		maxTouchSlot:  axisMaxTouchSlot,
		maxTrackingID: axisMaxTracking,
		maxPressure:   axisMaxPressure,
	}, nil
}

// Close closes the touchscreen device.
func (tsw *TouchscreenEventWriter) Close() error {
	firstErr := tsw.rw.Close()

	// Let go the virtual device if any.
	if tsw.virt != nil {
		if err := tsw.virt.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// NewMultiTouchWriter returns a new TouchEventWriter instance. numTouches is how many touches
// are going to be used by the TouchEventWriter.
func (tsw *TouchscreenEventWriter) NewMultiTouchWriter(numTouches int) (*TouchEventWriter, error) {
	if numTouches < 1 || numTouches > tsw.maxTouchSlot {
		return nil, errors.Errorf("requested %d touches; device only supports a max of %d touches", numTouches, tsw.maxTouchSlot+1)
	}

	tw := TouchEventWriter{tsw: tsw, touchStartTime: tsw.rw.nowFunc()}
	tw.initTouchState(numTouches)
	return &tw, nil
}

// NewSingleTouchWriter returns a new SingleTouchEventWriter instance.
// The difference between calling NewSingleTouchWriter() and NewMultiTouchWriter(1)
// is that NewSingleTouchWriter() has the extra helper Move() method.
func (tsw *TouchscreenEventWriter) NewSingleTouchWriter() (*SingleTouchEventWriter, error) {
	stw := SingleTouchEventWriter{TouchEventWriter{tsw: tsw, touchStartTime: tsw.rw.nowFunc()}}
	stw.initTouchState(1)
	return &stw, nil
}

// Width returns the width of the touchscreen device, in touchscreen coordinates.
// This is affected by the rotation of the screen.
func (tsw *TouchscreenEventWriter) Width() TouchCoord {
	if tsw.rotation == 90 || tsw.rotation == 270 {
		return tsw.height
	}
	return tsw.width
}

// Height returns the height of the touchscreen device, in touchscreen coordinates.
// This is affected by the rotation of the screen.
func (tsw *TouchscreenEventWriter) Height() TouchCoord {
	if tsw.rotation == 90 || tsw.rotation == 270 {
		return tsw.width
	}
	return tsw.height
}

// SetRotation changes the orientation of the touch screen's event to the
// specified degree. The locations of further touch events will be rotated by
// the specified rotation. It will return an error if the specified rotation is
// not supported.
func (tsw *TouchscreenEventWriter) SetRotation(rotation int) error {
	rotation = rotation % 360
	if rotation < 0 {
		rotation += 360
	}
	if rotation != 0 && rotation != 90 && rotation != 180 && rotation != 270 {
		return errors.Errorf("unsupported rotation: %d", rotation)
	}
	tsw.rotation = rotation
	return nil
}

// TouchCoordConverter manages the conversion between locations in DIP and
// the TouchCoord of the touchscreen.
type TouchCoordConverter struct {
	ScaleX float64
	ScaleY float64
}

// NewTouchCoordConverter creates a new TouchCoordConverter instance for the
// given size.
func (tsw *TouchscreenEventWriter) NewTouchCoordConverter(size coords.Size) *TouchCoordConverter {
	return &TouchCoordConverter{
		ScaleX: float64(tsw.Width()) / float64(size.Width),
		ScaleY: float64(tsw.Height()) / float64(size.Height),
	}
}

// ConvertLocation converts a location to TouchCoord.
func (tcc *TouchCoordConverter) ConvertLocation(l coords.Point) (x, y TouchCoord) {
	return TouchCoord(tcc.ScaleX * float64(l.X)), TouchCoord(tcc.ScaleY * float64(l.Y))
}

// TouchEventWriter supports injecting touch events into a touchscreen device.
type TouchEventWriter struct {
	tsw                       *TouchscreenEventWriter
	touches                   []TouchState
	touchStartTime            time.Time
	ended                     bool
	isBtnToolFingerEnabled    bool
	isBtnToolDoubleTapEnabled bool
}

// SingleTouchEventWriter supports injecting a single touch into a touchscreen device.
type SingleTouchEventWriter struct {
	TouchEventWriter
}

// TouchState contains the state of a single touch event.
type TouchState struct {
	tsw         *TouchscreenEventWriter
	slot        int32
	touchID     int32
	touchMinor  int32
	touchMajor  int32
	absPressure int32
	x           TouchCoord
	y           TouchCoord
}

// SetPos sets TouchState X and Y coordinates.
// X and Y must be between [0, touchscreen width) and [0, touchscreen height).
func (ts *TouchState) SetPos(x, y TouchCoord) error {
	if x < 0 || x >= ts.tsw.Width() || y < 0 || y >= ts.tsw.Height() {
		return errors.Errorf("coordinates (%d, %d) outside valid bounds [0, %d), [0, %d)",
			x, y, ts.tsw.Width(), ts.tsw.Height())
	}
	switch ts.tsw.rotation {
	case 90:
		x, y = ts.tsw.width-1-y, x
	case 180:
		x, y = ts.tsw.width-1-x, ts.tsw.height-1-y
	case 270:
		x, y = y, ts.tsw.height-1-x
	}
	ts.x = x
	ts.y = y
	return nil
}

// absInfo corresponds to a input_absinfo struct.
// Taken from: include/uapi/linux/input.h
type absInfo struct {
	value      uint32
	minimum    uint32
	maximum    uint32
	fuzz       uint32
	flat       uint32
	resolution uint32
}

// evIOCGAbs returns an encoded Event-Ioctl-Get-Absolute value to be used for ioctl().
// Similar to the EVIOCGABS found in include/uapi/linux/input.h
func evIOCGAbs(ev uint) uint {
	const sizeofAbsInfo = 0x24
	return ior('E', 0x40+ev, sizeofAbsInfo)
}

// evIOCSAbs sets an encoded Event-Ioctl-Set-Absolute value to be used for ioctl().
// Similar to the EVIOCSABS found in include/uapi/linux/input.h
func evIOCSAbs(ev uint) uint {
	const sizeofAbsInfo = 0x24
	return iow('E', 0xc0+ev, sizeofAbsInfo)
}

type kernelEventEntry struct {
	et  EventType
	ec  EventCode
	val int32
}

// Send sends all the multi-touch events to the kernel.
func (tw *TouchEventWriter) Send() error {
	// First send the multitouch event codes.
	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, touch.touchID},
			{EV_ABS, ABS_MT_POSITION_X, int32(touch.x)},
			{EV_ABS, ABS_MT_POSITION_Y, int32(touch.y)},
			{EV_ABS, ABS_MT_PRESSURE, touch.absPressure},
			{EV_ABS, ABS_MT_TOUCH_MAJOR, touch.touchMajor},
			{EV_ABS, ABS_MT_TOUCH_MINOR, touch.touchMinor},
		} {
			if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	// Then send the rest of the event codes.
	globalKernelEvents := []kernelEventEntry{
		{EV_KEY, BTN_TOUCH, 1},
		{EV_ABS, ABS_X, int32(tw.touches[0].x)},
		{EV_ABS, ABS_Y, int32(tw.touches[0].y)},
		{EV_ABS, ABS_PRESSURE, tw.touches[0].absPressure},
	}

	if tw.isBtnToolFingerEnabled {
		globalKernelEvents = append(globalKernelEvents, kernelEventEntry{et: EV_KEY, ec: BTN_TOOL_FINGER, val: int32(1)})
	}

	if tw.isBtnToolDoubleTapEnabled {
		globalKernelEvents = append(globalKernelEvents, kernelEventEntry{et: EV_KEY, ec: BTN_TOOL_DOUBLETAP, val: int32(1)})
	}

	for _, e := range globalKernelEvents {
		if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}
	tw.ended = false

	// And finally sync.
	return tw.tsw.rw.Sync()
}

// End injects a "touch lift" like if someone were lifting the finger or
// stylus from the surface. All active TouchStates are ended.
func (tw *TouchEventWriter) End() error {
	for _, touch := range tw.touches {
		for _, e := range []kernelEventEntry{
			{EV_ABS, ABS_MT_SLOT, touch.slot},
			{EV_ABS, ABS_MT_TRACKING_ID, -1},
		} {
			if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
				return err
			}
		}
	}

	globalEventsToEnd := []kernelEventEntry{
		{EV_ABS, ABS_PRESSURE, 0},
		{EV_KEY, BTN_TOUCH, 0},
	}

	if tw.isBtnToolFingerEnabled {
		globalEventsToEnd = append(globalEventsToEnd, kernelEventEntry{et: EV_KEY, ec: BTN_TOOL_FINGER, val: 0})
	}

	if tw.isBtnToolDoubleTapEnabled {
		globalEventsToEnd = append(globalEventsToEnd, kernelEventEntry{et: EV_KEY, ec: BTN_TOOL_DOUBLETAP, val: 0})
	}

	for _, e := range globalEventsToEnd {
		if err := tw.tsw.rw.Event(e.et, e.ec, e.val); err != nil {
			return err
		}
	}

	tw.ended = true
	tw.isBtnToolFingerEnabled = false
	tw.isBtnToolDoubleTapEnabled = false

	return tw.tsw.rw.Sync()
}

// Close cleans up TouchEventWriter. This method must be called after using it,
// possibly with the "defer" statement.
func (tw *TouchEventWriter) Close() {
	if !tw.ended {
		tw.End()
	}
}

// Swipe performs a swipe movement with an user defined number of touches. The touches are separated in the x
// coordinates by d. So for a 3 touch swipe, the initial touches will be (x0, y0), (x0+d, y0) and (x0+2d, y0).
// t represents how long the swipe should last. If t is less than 5 milliseconds, 5 milliseconds will be used instead.
// Swipe() does not call End(), allowing the user to concatenate multiple swipes together.
func (tw *TouchEventWriter) Swipe(ctx context.Context, x0, y0, x1, y1, d TouchCoord, touches int, t time.Duration) error {
	if len(tw.touches) < touches {
		return errors.Errorf("requested %d touches for swipe; got %d", touches, len(tw.touches))
	}
	steps := int(t/touchFrequency) + 1
	// A minimum of two touches are needed. One for the start point and another one for the end point.
	if steps < 2 {
		steps = 2
	}
	deltaX := float64(x1-x0) / float64(steps-1)
	deltaY := float64(y1-y0) / float64(steps-1)

	for i := 0; i < steps; i++ {
		x := x0 + TouchCoord(math.Round(deltaX*float64(i)))
		y := y0 + TouchCoord(math.Round(deltaY*float64(i)))

		for j := 0; j < touches; j++ {
			if err := tw.touches[j].SetPos(x+TouchCoord(j)*d, y); err != nil {
				return err
			}
		}

		if err := tw.Send(); err != nil {
			return err
		}

		if err := testing.Sleep(ctx, touchFrequency); err != nil {
			return errors.Wrap(err, "timeout while doing sleep")
		}
	}
	return nil
}

// touchCoordPoint represents a point, expressed in TouchCoords.
type touchCoordPoint struct {
	X, Y TouchCoord
}

// getPointsBetweenCoords returns all the coordinates between two points, spread out
// across the provided number of steps. Points are capped to the bounds of the touchscreen.
func getPointsBetweenCoords(ts *TouchscreenEventWriter, x0, y0, x1, y1 TouchCoord, steps int) []touchCoordPoint {
	// A minimum of two steps are needed. One for the start point and another one for the end point.
	stepsToUse := steps
	if stepsToUse < 2 {
		stepsToUse = 2
	}

	deltaX := float64(x1-x0) / float64(stepsToUse-1)
	deltaY := float64(y1-y0) / float64(stepsToUse-1)

	var result []touchCoordPoint
	for i := 0; i < stepsToUse; i++ {
		// Determine where the new point should be and keep it within the
		// bounds of the touchscreen.
		newX := x0 + TouchCoord(math.Round(deltaX*float64(i)))
		if newX < 0 {
			newX = 0
		} else if newX >= ts.Width() {
			newX = ts.Width() - 1
		}

		newY := y0 + TouchCoord(math.Round(deltaY*float64(i)))
		if newY < 0 {
			newY = 0
		} else if newY >= ts.Height() {
			newY = ts.Height() - 1
		}

		result = append(result, touchCoordPoint{
			X: newX,
			Y: newY,
		})
	}

	return result
}

// moveMultipleTouches moves a series of touches through multiple movements. Note that all touches
// must have the same number of movements. The number of touches is equal to len(pointsPerTouch) and
// the number of movements is equal to len(pointsPerTouch[0]).
func (tw *TouchEventWriter) moveMultipleTouches(ctx context.Context, pointsPerTouch ...[]touchCoordPoint) error {
	// If there are no items, exit.
	if len(pointsPerTouch) <= 0 {
		return nil
	}

	// All the points must have the same length.
	requiredLength := len(pointsPerTouch[0])
	for _, curTouch := range pointsPerTouch {
		if len(curTouch) != requiredLength {
			return errors.New("all pointsPerTouch must have the same length")
		}
	}

	// Move all the points through their coordinates at the same rate.
	for pointNum := 0; pointNum < requiredLength; pointNum++ {
		for touchNum, curTouch := range pointsPerTouch {
			if err := tw.touches[touchNum].SetPos(curTouch[pointNum].X, curTouch[pointNum].Y); err != nil {
				return err
			}
		}

		if err := tw.Send(); err != nil {
			return err
		}

		if err := testing.Sleep(ctx, touchFrequency); err != nil {
			return errors.Wrap(err, "timeout while doing sleep")
		}
	}

	return nil
}

// performPinchZoom performs a pinch zoom using the provided coordinates.
// A zoom in will start at the center and move points to the bottomLeft,
// and topRight. A zoom out will do the inverse.
func (tw *TouchEventWriter) performPinchZoom(ctx context.Context, center, bottomLeft, topRight coords.Point, t time.Duration, zoom ZoomType) error {
	// Ensure enough touches are provided.
	if len(tw.touches) < 2 {
		return errors.New("must have at least two touches to perform a zoom")
	}

	// Set up the points based on the zoom type.
	var leftFingerStart, leftFingerEnd coords.Point
	var rightFingerStart, rightFingerEnd coords.Point
	switch zoom {
	case ZoomIn:
		leftFingerStart = center
		leftFingerEnd = bottomLeft
		rightFingerStart = center
		rightFingerEnd = topRight
	case ZoomOut:
		leftFingerStart = bottomLeft
		leftFingerEnd = center
		rightFingerStart = topRight
		rightFingerEnd = center
	default:
		return errors.Errorf("invalid zoom provided: %v", zoom)
	}

	// Perform the zoom over a series of steps.
	steps := int(t/touchFrequency) + 1
	leftFingerPoints := getPointsBetweenCoords(tw.tsw, TouchCoord(leftFingerStart.X), TouchCoord(leftFingerStart.Y), TouchCoord(leftFingerEnd.X), TouchCoord(leftFingerEnd.Y), steps)
	rightFingerPoints := getPointsBetweenCoords(tw.tsw, TouchCoord(rightFingerStart.X), TouchCoord(rightFingerStart.Y), TouchCoord(rightFingerEnd.X), TouchCoord(rightFingerEnd.Y), steps)
	return tw.moveMultipleTouches(ctx, leftFingerPoints, rightFingerPoints)
}

// Zoom performs a pinch-to-zoom where the distance traveled to/from the
// provided center point is d for each finger.
func (tw *TouchEventWriter) Zoom(ctx context.Context, centerX, centerY, d TouchCoord, t time.Duration, zoom ZoomType) error {
	bottomLeft := coords.NewPoint(int(centerX)-int(d), int(centerY)+int(d))
	topRight := coords.NewPoint(int(centerX)+int(d), int(centerY)-int(d))
	center := coords.NewPoint(int(centerX), int(centerY))
	return tw.performPinchZoom(ctx, center, bottomLeft, topRight, t, zoom)
}

// ZoomRelativeToSize performs a pinch-to-zoom where the distance traveled, and
// center point are calculated based on the size of the Touch writer's
// dimensions. This function will attempt to use as much of the dimensions
// as possible in order to reliably trigger the zoom.
func (tw *TouchEventWriter) ZoomRelativeToSize(ctx context.Context, t time.Duration, zoom ZoomType) error {
	// Used to shrink the size of the writer in order to utilize as much
	// as possible without reaching the edges.
	sizeInset := int(math.Min(float64(tw.tsw.Width()), float64(tw.tsw.Height())) * .01)

	// Generate a rectangle that is a bit smaller than the
	// dimensions of the writer.
	writerDimensions := coords.NewRect(0, 0, int(tw.tsw.Width()), int(tw.tsw.Height()))
	insetWriterDimensions := writerDimensions.WithInset(sizeInset, sizeInset)

	// Calculate the relevant points which use up as much of the writer's
	// dimensions as possible.
	center := insetWriterDimensions.CenterPoint()
	bottomLeft := insetWriterDimensions.BottomLeft()
	topRight := insetWriterDimensions.TopRight()
	return tw.performPinchZoom(ctx, center, bottomLeft, topRight, t, zoom)
}

// DoubleSwipe performs a swipe movement with two touches. One is from x0/y0 to x1/y1, and the other is x0+d/y0 to x1+d/y1.
// t represents how long the swipe should last.
// If t is less than 5 milliseconds, 5 milliseconds will be used instead.
// DoubleSwipe() does not call End(), allowing the user to concatenate multiple swipes together.
func (tw *TouchEventWriter) DoubleSwipe(ctx context.Context, x0, y0, x1, y1, d TouchCoord, t time.Duration) error {
	return tw.Swipe(ctx, x0, y0, x1, y1, d, 2, t)
}

// SetSize sets the major/minor appropriately for all touches.
func (tw *TouchEventWriter) SetSize(ctx context.Context, major, minor int32) error {
	if major < 0 || minor < 0 {
		return errors.Errorf("must be positive; got: major=%v, minor=%v", major, minor)
	} else if major < minor {
		return errors.Errorf("major must be greater than or equal to minor; got: major=%v, minor=%v", major, minor)
	}

	for idx := range tw.touches {
		tw.touches[idx].touchMajor = major
		tw.touches[idx].touchMinor = minor
	}

	return nil
}

// SetIsBtnToolFinger Sets the state of the BTN_TOOL_FINGER flag.
func (tw *TouchEventWriter) SetIsBtnToolFinger(isEnabled bool) {
	tw.isBtnToolFingerEnabled = isEnabled
}

// SetIsBtnToolDoubleTap Sets the state of the BTN_TOOL_DOUBLETAP flag.
func (tw *TouchEventWriter) SetIsBtnToolDoubleTap(isEnabled bool) {
	tw.isBtnToolDoubleTapEnabled = isEnabled
}

// SetPressure sets the pressure of each touch.
func (tw *TouchEventWriter) SetPressure(pressure int32) error {
	if pressure < 0 {
		return errors.New("pressure must be greater than 0")
	}

	for idx := range tw.touches {
		tw.touches[idx].absPressure = pressure
	}

	return nil
}

// Move injects a touch event at x and y touchscreen coordinates. This is applied
// only to the first TouchState. Calling this function is equivalent to:
//  ts := touchEventWriter.TouchState(0)
//  ts.SetPos(x, y)
//  ts.Send()
func (stw *SingleTouchEventWriter) Move(x, y TouchCoord) error {
	if err := stw.touches[0].SetPos(x, y); err != nil {
		return err
	}
	return stw.Send()
}

// LongPressAt injects a touch event at (x, y) touchscreen coordinates and wait
// a bit to simulate a touch long press. The wait time should be longer than
// chrome's default long press wait time, which is 500ms.
// See ui/events/gesture_detection/gesture_detector.cc in chromium.
func (stw *SingleTouchEventWriter) LongPressAt(ctx context.Context, x, y TouchCoord) error {
	if err := stw.Move(x, y); err != nil {
		return err
	}

	return testing.Sleep(ctx, 1*time.Second)
}

// SetSize sets the major/minor appropriately for single touch events. Sets the
// major/minor.
func (stw *SingleTouchEventWriter) SetSize(ctx context.Context, major, minor int32) error {
	if len(stw.touches) != 1 {
		return errors.New("expected touches size to be 1, is ")
	}
	if major < 0 || minor < 0 {
		return errors.New("major and minor must be positive")
	} else if major < minor {
		return errors.New("major must be greater than or equal to minor")
	}
	stw.touches[0].touchMajor = major
	stw.touches[0].touchMinor = minor
	return nil
}

// Swipe performs a swipe movement from x0/y0 to x1/y1.
// t represents how long the swipe should last.
// If t is less than 5 milliseconds, 5 milliseconds will be used instead.
// Swipe() does not call End(), allowing the user to concatenate multiple swipes together.
func (stw *SingleTouchEventWriter) Swipe(ctx context.Context, x0, y0, x1, y1 TouchCoord, t time.Duration) error {
	steps := int(t/touchFrequency) + 1
	// A minimum of two touches are needed. One for the start point and another one for the end point.
	if steps < 2 {
		steps = 2
	}
	deltaX := float64(x1-x0) / float64(steps-1)
	deltaY := float64(y1-y0) / float64(steps-1)

	for i := 0; i < steps; i++ {
		x := x0 + TouchCoord(math.Round(deltaX*float64(i)))
		y := y0 + TouchCoord(math.Round(deltaY*float64(i)))
		if err := stw.Move(x, y); err != nil {
			return err
		}

		if err := testing.Sleep(ctx, touchFrequency); err != nil {
			return errors.Wrap(err, "timeout while doing sleep")
		}
	}
	return nil
}

// TouchState returns a TouchState. touchIndex is touch to get.
// One TouchState represents the state of a single touch.
func (tw *TouchEventWriter) TouchState(touchIndex int) *TouchState {
	return &tw.touches[touchIndex]
}

func (tw *TouchEventWriter) initTouchState(numTouches int) {
	// Values taken from "dumps" on an Eve device.
	// Spec says pressure is in arbitrary units. A value around 25% of the max value seems to be "normal".
	// TouchMajor and TouchMinor were also taken from "dumps".
	const (
		defaultTouchMajor = 5
		defaultTouchMinor = 5
	)
	defaultPressure := int32(tw.tsw.maxPressure/4) + 1

	tw.touches = make([]TouchState, numTouches)

	for i := 0; i < numTouches; i++ {
		tw.touches[i].tsw = tw.tsw
		tw.touches[i].absPressure = defaultPressure
		tw.touches[i].touchMajor = defaultTouchMajor
		tw.touches[i].touchMinor = defaultTouchMinor
		tw.touches[i].touchID = tw.tsw.nextTouchID
		tw.touches[i].slot = int32(i)

		tw.tsw.nextTouchID = (tw.tsw.nextTouchID + 1) % int32(tw.tsw.maxTrackingID)
	}
}
