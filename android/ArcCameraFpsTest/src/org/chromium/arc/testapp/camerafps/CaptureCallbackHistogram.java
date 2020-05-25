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
import android.os.SystemClock;

class CaptureCallbackHistogram extends CaptureCallback {

    // Maximum duration between two frames.
    private static final int HISTOGRAM_MAX = 1024;
    // Frame considered dropped if it is more than 50% late.
    private static final float FRAME_DROP_FACTOR = 1.5f;

    // Histogram of duration between two frames.
    private int[] histogram = new int[HISTOGRAM_MAX];
    // Last reocrded frame timestamp.
    private long mLastTimeStamp = 0;
    // Total number of frames.
    private long mNumFrames = 0;
    // Total number of dropped frames.
    private long mNumDroppedFrames = 0;
    // Target frame duration in milliseconds, i.e., time between two frames.
    // Default: 30 FPS -> 33 ms
    private int mTargetFrameDuration = 33;
    // Sum of all snapshot durations.
    private long mSnapshotTimeSum = 0;
    // Total number of snapshots.
    private long mNumSnapshots = 0;
    // Time it took (in ms) to take the last snapshot.
    private long mLastSnapshotTime = -1;

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

    public long getNumFrames() {
        return mNumFrames;
    }

    public long getNumDroppedFrames() {
        return mNumDroppedFrames;
    }

    // Returns the time it took to take a snapshot on average.
    public double getAverageSnapshotTime() {
        if (mNumSnapshots == 0) {
            return 0.0;
        } else {
            return ((double) mSnapshotTimeSum) / mNumSnapshots;
        }
    }

    public long getLastSnapshotTime() {
        return mLastSnapshotTime;
    }

    // Store the time it took to take one snapshot.
    public void addSnapshotTime(long time) {
        mNumSnapshots++;
        mSnapshotTimeSum += time;
        mLastSnapshotTime = time;
    }

    // Callback is fired when a frame arrives.
    @Override
    public void onCaptureCompleted(
            CameraCaptureSession session, CaptureRequest request, TotalCaptureResult result) {
        mNumFrames++;

        if (mLastTimeStamp == 0) {
            mLastTimeStamp = SystemClock.elapsedRealtime();
        } else {
            int duration = (int) (SystemClock.elapsedRealtime() - mLastTimeStamp);
            mLastTimeStamp = SystemClock.elapsedRealtime();

            if (duration < HISTOGRAM_MAX - 1) {
                histogram[duration]++;
            } else {
                histogram[HISTOGRAM_MAX - 1]++;
            }

            if (mTargetFrameDuration > 0 && duration > FRAME_DROP_FACTOR * mTargetFrameDuration) {
                mNumDroppedFrames++;
            }
        }
    }

    // Reset histogram to all zeros.
    public void resetHistogram() {
        histogram = new int[HISTOGRAM_MAX];
        mLastTimeStamp = 0;
        mNumDroppedFrames = 0;
        mNumFrames = 0;
        mSnapshotTimeSum = 0;
        mNumSnapshots = 0;
        mLastSnapshotTime = -1;
    }

    public void setTargetFrameDuration(int duration) {
        mTargetFrameDuration = duration;
    }
}
