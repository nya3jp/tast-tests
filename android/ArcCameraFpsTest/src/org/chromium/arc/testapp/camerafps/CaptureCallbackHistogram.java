/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.camerafps;

import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCaptureSession.CaptureCallback;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.TotalCaptureResult;
import android.util.Log;

class CaptureCallbackHistogram extends CaptureCallback {

    // Maximum duration between two frames.
    private static final int HISTOGRAM_MAX = 1024;
    // Histogram of duration between two frames.
    private int[] histogram = new int[HISTOGRAM_MAX];
    // Last reocrded frame timestamp.
    private long mLastTimeStamp = 0;

    // Get histogram as string.
    public String getHistogramString() {
       StringBuilder sb = new StringBuilder();
       sb.append("[");
       for (int i = 0; i < HISTOGRAM_MAX - 1; ++i) {
        sb.append(histogram[i]);
        sb.append(", ");
       }
       sb.append(histogram[HISTOGRAM_MAX - 1]);
       sb.append("]");
       return sb.toString();
    }

    // Callback is fired when a frame arrives.
    @Override
    public void onCaptureCompleted(CameraCaptureSession session,
            CaptureRequest request, TotalCaptureResult result) {
        if (mLastTimeStamp == 0) {
            mLastTimeStamp = System.currentTimeMillis();
        } else {
            int duration = (int) (System.currentTimeMillis() - mLastTimeStamp);
            mLastTimeStamp = System.currentTimeMillis();

            if (duration < HISTOGRAM_MAX - 1) {
                histogram[duration]++;
            } else {
                histogram[HISTOGRAM_MAX - 1]++;
            }
        }
    }

    // Reset histogram to all zeros.
    public void resetHistogram() {
        histogram = new int[HISTOGRAM_MAX];
    }

}