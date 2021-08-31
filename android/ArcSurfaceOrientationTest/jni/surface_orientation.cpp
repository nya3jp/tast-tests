// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/*
 * Draw a buffer that is reoriented using one of the transform given by the
 * native java method. 4 colored blocks will be drawn to the buffer and then
 * Chrome should reoriente them according to the transform passed.
 */

#include <map>

#include <jni.h>

#include <android/native_window.h>
#include <android/native_window_jni.h>

extern "C" JNIEXPORT void JNICALL
Java_org_chromium_arc_testapp_surfaceorientation_MainActivity_nativeRenderToSurface(
        JNIEnv* env, jobject /* jthis */, jobject surface, jint fixedTransform);

namespace {

constexpr uint32_t colorRGB(uint8_t r, uint8_t g, uint8_t b) {
    // The system requires the pixels to be set with RGBA encoding so use
    // opaque value of 255 for the A value.
    constexpr uint8_t kOpaqueAValue = 255;
    return static_cast<uint32_t>(
            (kOpaqueAValue << 24) | (b << 16) | (g << 8) | (r << 0));
}

constexpr uint32_t kGray = colorRGB(127, 127, 127);

constexpr uint32_t kRed = colorRGB(255, 0, 0);
constexpr uint32_t kGreen = colorRGB(0, 255, 0);
constexpr uint32_t kBlue = colorRGB(0, 0, 255);
constexpr uint32_t kYellow = colorRGB(255, 255, 0);

enum class Quadrant {
    TOP_LEFT,
    TOP_RIGHT,
    BOTTOM_LEFT,
    BOTTOM_RIGHT
};

constexpr std::map<Quadrant, uint32_t> quadrantToColor {
    {Quadrant::TOP_LEFT, kRed},
    {Quadrant::TOP_RIGHT, kGreen},
    {Quadrant::BOTTOM_LEFT, kBlue},
    {Quadrant::BOTTOM_RIGHT, kYellow}
};

constexpr uint32_t kClearColor = kGray;

struct ANativeWindowDeleter {
    void operator()(ANativeWindow* ptr) const {
        if (ptr) {
            ANativeWindow_release(ptr);
        }
    }
};
using UniqueANativeWindow = std::unique_ptr<
        ANativeWindow, ANativeWindowDeleter>;

class BufferWriter {
public:
    BufferWriter(ANativeWindow_Buffer &buffer)
          : bits_(static_cast<uint32_t*>(buffer.bits)), width_(buffer.width),
          height_(buffer.height), stride_(buffer.stride) {}

    // Set a quadrant to its color according to quadrantToColor map
    void setQuadrantToDefaultColor(Quadrant quadrant) {
        int halfWidth = width_ / 2;
        int halfHeight = height_ / 2;
        switch(quadrant) {
        case Quadrant::TOP_LEFT:
            setRectToColor(0, 0, halfWidth, halfHeight,
                           quadrantToColor[Quadrant::TOP_LEFT]);
            break;
        case Quadrant::TOP_RIGHT:
            setRectToColor(halfWidth, 0, width_, halfHeight,
                           quadrantToColor[Quadrant::TOP_RIGHT]);
            break;
        case Quadrant::BOTTOM_LEFT:
            setRectToColor(0, halfHeight, halfWidth, height_,
                           quadrantToColor[Quadrant::BOTTOM_LEFT]);
            break;
        default /* Quadrant::BOTTOM_RIGHT */:
            setRectToColor(halfWidth, halfHeight, width_, height_,
                           quadrantToColor[Quadrant::BOTTOM_RIGHT]);
        }
    }

    void clear() { setRect(0, 0, width_, height_, kClearColor); }

private:
    uint32_t* const bits_;
    const int width_;
    const int height_;
    const int stride_;

    void setPixelToColor(int x, int y, uint32_t color) {
        *(bits_ + x + y * stride_) = color;
    }

    void setRectToColor(int left, int top, int right, int bottom,
                        uint32_t color) {
        for (int y = top; y < bottom; y++) {
            for (int x = left; x < right; x++) {
                setPixelToColor(x, y, color);
            }
        }
    }
};

void drawBuffer(ANativeWindow_Buffer &buffer) {
    BufferWriter writer(buffer);

    writer.clear();

    writer.setQuadrantToDefaultColor(Quadrant::TOP_LEFT);
    writer.setQuadrantToDefaultColor(Quadrant::TOP_RIGHT);
    writer.setQuadrantToDefaultColor(Quadrant::BOTTOM_LEFT);
    writer.setQuadrantToDefaultColor(Quadrant::BOTTOM_RIGHT);
}

} // namespace

Java_org_chromium_arc_testapp_surfaceorientation_MainActivity_nativeRenderToSurface(
        JNIEnv* env, jobject /* jthis */, jobject surface, jint transform) {
    UniqueANativeWindow window{ANativeWindow_fromSurface(env, surface)};
    if (window == nullptr) {
        return;
    }

    ANativeWindow_Buffer buffer;
    if (ANativeWindow_lock(window.get(), &buffer, nullptr) != 0) {
        return;
    }

    // TODO: Do I need to call this function for anything? AKA, does the pixel
    // format need to be set?
    ANativeWindow_setBuffersGeometry(window.get(), 0, 0,
                                     WINDOW_FORMAT_RGBX_8888);

    native_window_set_buffers_transform(window.get(), transform);

    drawBuffer(buffer);

    ANativeWindow_unlockAndPost(window.get());
}