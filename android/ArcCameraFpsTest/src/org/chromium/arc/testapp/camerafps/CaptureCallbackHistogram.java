/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.camerafps;

import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCaptureSession.CaptureCallback;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.CaptureResult;
import android.hardware.camera2.TotalCaptureResult;
import android.os.SystemClock;

import java.util.HashMap;
import java.util.Map;

class CaptureCallbackHistogram extends CaptureCallback {

    // Maximum duration between two frames.
    private static final int HISTOGRAM_MAX = 1024;
    // Frame considered dropped if it is more than 50% late.
    private static final float FRAME_DROP_FACTOR = 1.5f;

    // Histogram of duration (ms) between two frames.
    private int[] mHistogram = new int[HISTOGRAM_MAX];
    // Histogram of duration (ms) between two frames (sensor timestamps).
    private int[] mHistogramSensor = new int[HISTOGRAM_MAX];
    // Last recorded frame timestamp (Java callback time).
    private long mLastTimeStampJava = 0;
    // Last recorded frame timestamp (sensor timestamp).
    private long mLastTimeStampSensor = 0;
    // Total number of frames.
    private long mNumFrames = 0;
    // Total number of dropped frames (Java callback).
    private long mNumDroppedFramesJava = 0;
    // Total number of dropped frames (based on sensor timestamps).
    private long mNumDroppedFramesSensor = 0;
    // Target frame duration in milliseconds, i.e., time between two frames.
    // Default: 30 FPS -> 33 ms
    private int mTargetFrameDuration = 33;
    // Histogram of pipeline latency (ns).
    private Map<Long, Integer> mPipelineLatencyHistogram = new HashMap<Long, Integer>();

    // Get Java callback duration histogram as string.
    public String getHistogramString() {
        StringBuilder sb = new StringBuilder();
        sb.append("[");
        for (int i = 0; i < HISTOGRAM_MAX - 1; ++i) {
            sb.append(mHistogram[i]);
            sb.append(", ");
        }
        sb.append(mHistogram[HISTOGRAM_MAX - 1]);
        sb.append("]");
        return sb.toString();
    }

    // Get duration histogram based on sensor timestamps as string.
    public String getHistogramSensorString() {
        StringBuilder sb = new StringBuilder();
        sb.append("[");
        for (int i = 0; i < HISTOGRAM_MAX - 1; ++i) {
            sb.append(mHistogramSensor[i]);
            sb.append(", ");
        }
        sb.append(mHistogramSensor[HISTOGRAM_MAX - 1]);
        sb.append("]");
        return sb.toString();
    }

    public String getLatencyHistogramString() {
        StringBuilder sb = new StringBuilder();
        sb.append("{");
        synchronized(mPipelineLatencyHistogram) {
            for (Map.Entry<Long, Integer> entry : mPipelineLatencyHistogram.entrySet()) {
                sb.append(entry.getKey());
                sb.append(": ");
                sb.append(entry.getValue());
                sb.append(", ");
            }
        }
        sb.append("}");
        return sb.toString();
    }

    public long getNumFrames() {
        return mNumFrames;
    }

    public long getNumDroppedFrames() {
        return mNumDroppedFramesJava;
    }

    public long getNumDroppedFramesSensor() {
        return mNumDroppedFramesSensor;
    }

    // Callback is fired when a frame arrives.
    @Override
    public void onCaptureCompleted(
            CameraCaptureSession session, CaptureRequest request, TotalCaptureResult result) {
        mNumFrames++;

        // Record Java callback timstamp.
        long timeStampJava = SystemClock.elapsedRealtimeNanos();
        if (mLastTimeStampJava == 0) {
            mLastTimeStampJava = timeStampJava;
        } else {
            // Convert to nanoseconds to milliseconds.
            int duration = (int) (timeStampJava - mLastTimeStampJava) / 1000000;
            mLastTimeStampJava = timeStampJava;

            if (duration < HISTOGRAM_MAX - 1) {
                mHistogram[duration]++;
            } else {
                mHistogram[HISTOGRAM_MAX - 1]++;
            }

            if (mTargetFrameDuration > 0 && duration > FRAME_DROP_FACTOR * mTargetFrameDuration) {
                mNumDroppedFramesJava++;
            }
        }

        // Record sensor callback timestamp.
        long timeStampSensor = result.get(CaptureResult.SENSOR_TIMESTAMP);
        if (mLastTimeStampSensor == 0) {
            mLastTimeStampSensor = timeStampSensor;
        } else {
            // Convert nanoseconds to milliseconds.
            int duration = (int) (timeStampSensor - mLastTimeStampSensor) / 1000000;
            mLastTimeStampSensor = timeStampSensor;

            if (duration < HISTOGRAM_MAX - 1) {
                mHistogramSensor[duration]++;
            } else {
                mHistogramSensor[HISTOGRAM_MAX - 1]++;
            }

            // Sensor timestamps are in nanoseconds.
            if (mTargetFrameDuration > 0 && duration > FRAME_DROP_FACTOR * mTargetFrameDuration) {
                mNumDroppedFramesSensor++;
            }
        }

        // Compute pipeline latency and add to histogram.
        synchronized(mPipelineLatencyHistogram) {
            long latency = (timeStampJava - timeStampSensor) / 1000000;
            if (!mPipelineLatencyHistogram.containsKey(latency)) {
                mPipelineLatencyHistogram.put(latency, 1);
            } else {
                mPipelineLatencyHistogram.put(latency, mPipelineLatencyHistogram.get(latency) + 1);
            }
        }
    }

    // Reset histogram to all zeros.
    public void resetHistogram() {
        mHistogram = new int[HISTOGRAM_MAX];
        mHistogramSensor = new int[HISTOGRAM_MAX];
        mLastTimeStampJava = 0;
        mLastTimeStampSensor = 0;
        mNumDroppedFramesJava = 0;
        mNumDroppedFramesSensor = 0;
        mNumFrames = 0;
        mPipelineLatencyHistogram = new HashMap<Long, Integer>();
    }

    public void setTargetFrameDuration(int duration) {
        mTargetFrameDuration = duration;
    }
}
