/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.camerafps;

import android.util.Log;

import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCaptureSession.CaptureCallback;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.CaptureResult;
import android.hardware.camera2.TotalCaptureResult;
import android.os.SystemClock;

import java.util.Arrays;
import java.util.concurrent.CountDownLatch;
import java.util.HashMap;
import java.util.Map;

class CaptureCallbackHistogram extends CaptureCallback {

    private static final String TAG = "CaptureCallbackHistogram";

    // Maximum duration between two frames.
    private static final int HISTOGRAM_MAX = 1024;
    // Frame considered dropped if it is more than 50% late.
    private static final float FRAME_DROP_FACTOR = 1.5f;

    // Histogram of duration (ms) between two frames (Java callback).
    private int[] mHistogramJava = new int[HISTOGRAM_MAX];
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
    // Sum of all snapshot durations.
    private long mSnapshotTimeSum = 0;
    // Total number of snapshots.
    private long mNumSnapshots = 0;
    // Time it took (in ms) to take the last snapshot.
    private long mLastSnapshotTime = -1;
    // Histogram of pipeline latency (ns).
    private Map<Long, Integer> mPipelineLatencyHistogram = new HashMap<Long, Integer>();
    // Will be decremented when a frame arrives.
    private CountDownLatch mFrameSignal = new CountDownLatch(1);

    private String buildHistogramString(int[] histogram) {
        String result;
        synchronized(histogram) {
            result = Arrays.toString(histogram);
        }
        return result;
    }

    // Get Java callback duration histogram as string.
    public String getHistogramJavaString() {
        return buildHistogramString(mHistogramJava);
    }

    // Get duration histogram based on sensor timestamps as string.
    public String getHistogramSensorString() {
        return buildHistogramString(mHistogramSensor);
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

    public long getNumDroppedFramesJava() {
        return mNumDroppedFramesJava;
    }

    public long getNumDroppedFramesSensor() {
        return mNumDroppedFramesSensor;
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

    // Add a Java callback timestamp to the histogram.
    private void recordJavaTimeStamp(long timeStampJava) {
        if (mLastTimeStampJava == 0) {
            mLastTimeStampJava = timeStampJava;
        } else {
            // Convert nanoseconds to milliseconds.
            int duration = (int) (timeStampJava - mLastTimeStampJava) / 1000000;
            mLastTimeStampJava = timeStampJava;

            synchronized(mHistogramJava) {
                if (duration < HISTOGRAM_MAX - 1) {
                    mHistogramJava[duration]++;
                } else {
                    mHistogramJava[HISTOGRAM_MAX - 1]++;
                }
            }

            if (mTargetFrameDuration > 0 && duration > FRAME_DROP_FACTOR * mTargetFrameDuration) {
                mNumDroppedFramesJava++;
            }
        }
    }

    // Add a sensor timestamp to the histogram.
    private void recordSensorTimeStamp(long timeStampSensor) {
        if (mLastTimeStampSensor == 0) {
            mLastTimeStampSensor = timeStampSensor;
        } else {
            if (timeStampSensor < mLastTimeStampSensor) {
                Log.e(TAG, "Out-of-order sensor timestamps: " + timeStampSensor + ", last recorded "
                        + "timestamp = " + mLastTimeStampSensor);
                return;
            }
            // Convert nanoseconds to milliseconds.
            long duration = (timeStampSensor - mLastTimeStampSensor) / 1000000L;
            mLastTimeStampSensor = timeStampSensor;

            synchronized(mHistogramSensor) {
                if (duration < HISTOGRAM_MAX - 1) {
                    mHistogramSensor[(int) duration]++;
                } else {
                    mHistogramSensor[HISTOGRAM_MAX - 1]++;
                }
            }

            // Sensor timestamps are in nanoseconds.
            if (mTargetFrameDuration > 0 && duration > FRAME_DROP_FACTOR * mTargetFrameDuration) {
                mNumDroppedFramesSensor++;
            }
        }
    }

    // Callback is fired when a frame arrives.
    @Override
    public void onCaptureCompleted(
            CameraCaptureSession session, CaptureRequest request, TotalCaptureResult result) {
        mFrameSignal.countDown();
        mNumFrames++;

        // Record Java callback timestamp.
        long timeStampJava = SystemClock.elapsedRealtimeNanos();
        recordJavaTimeStamp(timeStampJava);

        // Record sensor callback timestamp.
        long timeStampSensor = result.get(CaptureResult.SENSOR_TIMESTAMP);
        recordSensorTimeStamp(timeStampSensor);

        // Compute pipeline latency and add to histogram.
        // TODO(b/160650453): Latency histogram values on ARCVM will be shifted by fixed offset
        // until host-guest timestamp issues are resolved.
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
        mHistogramJava = new int[HISTOGRAM_MAX];
        mHistogramSensor = new int[HISTOGRAM_MAX];
        mLastTimeStampJava = 0;
        mLastTimeStampSensor = 0;
        mNumDroppedFramesJava = 0;
        mNumDroppedFramesSensor = 0;
        mNumFrames = 0;
        mSnapshotTimeSum = 0;
        mNumSnapshots = 0;
        mLastSnapshotTime = -1;
        mPipelineLatencyHistogram = new HashMap<Long, Integer>();
        mFrameSignal = new CountDownLatch(1);
    }

    public void setTargetFrameDuration(int duration) {
        mTargetFrameDuration = duration;
    }

    public void waitForFirstFrame() throws InterruptedException {
        mFrameSignal.await();
    }
}
