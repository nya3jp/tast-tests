/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.camerafps;

import android.app.Activity;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Bundle;
import android.util.Log;
import android.util.Size;

public class CameraActivity extends Activity {

    private static final String TAG = "ArcCameraFpsTest";

    private static final String ACTION_GET_HISTOGRAM =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_HISTOGRAM";
    private static final String ACTION_GET_PREVIEW_SIZE =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_PREVIEW_SIZE";
    private static final String ACTION_GET_RECORDING_SIZE =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_RECORDING_SIZE";
    private static final String ACTION_RESET_HISTOGRAM =
            "org.chromium.arc.testapp.camerafps.ACTION_RESET_HISTOGRAM";
    private static final String ACTION_SET_TARGET_FPS =
            "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_FPS";
    private static final String ACTION_SET_TARGET_RESOLUTION =
            "org.chromium.arc.testapp.camerafps.ACTION_SET_TARGET_RESOLUTION";
    private static final String ACTION_START_RECORDING =
            "org.chromium.arc.testapp.camerafps.ACTION_START_RECORDING";
    private static final String ACTION_STOP_RECORDING =
            "org.chromium.arc.testapp.camerafps.ACTION_STOP_RECORDING";

    private static final String KEY_FPS = "fps";
    private static final int DEFAULT_FPS = 0;
    private static final String KEY_WIDTH = "width";
    private static final int DEFAULT_WIDTH = 0;
    private static final String KEY_HEIGHT = "height";
    private static final int DEFAULT_HEIGHT = 0;

    // A histogram  of frame durations (time between two frames).
    private final CaptureCallbackHistogram mHistogram = new CaptureCallbackHistogram();
    // The camera fragment provides preview + recording functionality.
    private Camera2VideoFragment mCameraFragment = null;

    private BroadcastReceiver mReceiver =
            new BroadcastReceiver() {
                @Override
                public void onReceive(Context context, Intent intent) {
                    try {
                        switch (intent.getAction()) {
                            case ACTION_GET_HISTOGRAM:
                                setResultData(mHistogram.getHistogramString());
                                break;
                            case ACTION_GET_PREVIEW_SIZE:
                                setResultData(mCameraFragment.getPreviewSize());
                                break;
                            case ACTION_GET_RECORDING_SIZE:
                                setResultData(mCameraFragment.getRecordingSize());
                                break;
                            case ACTION_RESET_HISTOGRAM:
                                mHistogram.resetHistogram();
                                break;
                            case ACTION_SET_TARGET_FPS:
                                int fps = intent.getIntExtra(KEY_FPS, DEFAULT_FPS);
                                mCameraFragment.setTargetFps(
                                        fps == DEFAULT_FPS ? null : fps);
                                mCameraFragment.startPreview();
                                break;
                            case ACTION_SET_TARGET_RESOLUTION:
                                int width = intent.getIntExtra(KEY_WIDTH, DEFAULT_WIDTH);
                                int height = intent.getIntExtra(
                                        KEY_HEIGHT, DEFAULT_HEIGHT);
                                if (width == DEFAULT_WIDTH || height == DEFAULT_HEIGHT) {
                                    mCameraFragment.setTargetResolution(null);
                                } else {
                                    mCameraFragment.setTargetResolution(
                                            new Size(width, height));
                                }
                                mCameraFragment.startPreview();
                                break;
                            case ACTION_START_RECORDING:
                                String filename = mCameraFragment.startRecordingVideo();
                                setResultData(filename);
                                break;
                            case ACTION_STOP_RECORDING:
                                mCameraFragment.stopRecordingVideo();
                                break;
                        }
                        setResultCode(Activity.RESULT_OK);
                    } catch (Exception e) {
                        setResultCode(Activity.RESULT_CANCELED);
                        setResultData(e.toString());
                        Log.e(TAG, "Error in " + intent.getAction(), e);
                    }
                }
            };

    private static IntentFilter getFilter() {
        IntentFilter filter = new IntentFilter();
        filter.addAction(ACTION_GET_HISTOGRAM);
        filter.addAction(ACTION_GET_PREVIEW_SIZE);
        filter.addAction(ACTION_GET_RECORDING_SIZE);
        filter.addAction(ACTION_RESET_HISTOGRAM);
        filter.addAction(ACTION_SET_TARGET_FPS);
        filter.addAction(ACTION_SET_TARGET_RESOLUTION);
        filter.addAction(ACTION_START_RECORDING);
        filter.addAction(ACTION_STOP_RECORDING);
        return filter;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        mCameraFragment = new Camera2VideoFragment(mHistogram);
        setContentView(R.layout.activity_camera);
        getFragmentManager().beginTransaction()
                .replace(R.id.container, mCameraFragment)
                .commit();
        this.registerReceiver(mReceiver, getFilter());
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        this.unregisterReceiver(mReceiver);
    }

}