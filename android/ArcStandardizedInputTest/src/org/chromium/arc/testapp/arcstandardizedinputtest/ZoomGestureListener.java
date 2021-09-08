/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.view.ScaleGestureDetector;

class ZoomGestureListener extends ScaleGestureDetector.SimpleOnScaleGestureListener {
    OnZoomListener mZoomInListener;
    OnZoomListener mZoomOutListener;
    OnZoomListener mZoomDebugListener;

    public ZoomGestureListener(
            OnZoomListener zoomInListener,
            OnZoomListener zoomOutListener,
            OnZoomListener zoomDebugListener) {
        mZoomInListener = zoomInListener;
        mZoomOutListener = zoomOutListener;
        mZoomDebugListener = zoomDebugListener;
    }

    @Override
    public void onScaleEnd(ScaleGestureDetector detector) {
        // Always fire the debug listener.
        mZoomDebugListener.zoom(detector.getScaleFactor());

        // Only fire the actual events if a change occurred.
        if (detector.getScaleFactor() > 1) {
            mZoomInListener.zoom(detector.getScaleFactor());
        } else if (detector.getScaleFactor() < 1) {
            mZoomOutListener.zoom(detector.getScaleFactor());
        }
    }
}
