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

    private static final String ACTION_GET_AVG_SNAPSHOT_TIME =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_AVG_SNAPSHOT_TIME";
    private static final String ACTION_GET_CAMERA_CLOSE_TIME =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_CLOSE_TIME";
    private static final String ACTION_GET_CAMERA_OPEN_TIME =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_OPEN_TIME";
    private static final String ACTION_GET_HISTOGRAM =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_HISTOGRAM";
    private static final String ACTION_GET_LAST_SNAPSHOT_TIME =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_LAST_SNAPSHOT_TIME";
    private static final String ACTION_GET_NUM_FRAMES =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_FRAMES";
    private static final String ACTION_GET_NUM_DROPPED_FRAMES =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_NUM_DROPPED_FRAMES";
    private static final String ACTION_GET_PREVIEW_RESOLUTIONS =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_PREVIEW_RESOLUTIONS";
    private static final String ACTION_GET_PREVIEW_SIZE =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_PREVIEW_SIZE";
    private static final String ACTION_GET_RECORDING_RESOLUTIONS =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_RECORDING_RESOLUTIONS";
    private static final String ACTION_GET_RECORDING_SIZE =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_RECORDING_SIZE";
    private static final String ACTION_GET_SNAPSHOT_RESOLUTIONS =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_SNAPSHOT_RESOLUTIONS";
    private static final String ACTION_GET_SNAPSHOT_SIZE =
            "org.chromium.arc.testapp.camerafps.ACTION_GET_SNAPSHOT_SIZE";
    private static final String ACTION_RESET_CAMERA =
            "org.chromium.arc.testapp.camerafps.ACTION_RESET_CAMERA";
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
    private static final String ACTION_TAKE_PHOTO =
            "org.chromium.arc.testapp.camerafps.ACTION_TAKE_PHOTO";

    private static final String KEY_FPS = "fps";
    private static final int DEFAULT_FPS = 30;

    // A resolution of 0x0 falls back to the maximum supported resolution.
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
                            case ACTION_GET_AVG_SNAPSHOT_TIME:
                                setResultData(Double.toString(mHistogram.getAverageSnapshotTime()));
                                break;
                            case ACTION_GET_CAMERA_CLOSE_TIME:
                                setResultData(Long.toString(mCameraFragment.getCameraCloseTime()));
                                break;
                            case ACTION_GET_CAMERA_OPEN_TIME:
                                setResultData(Long.toString(mCameraFragment.getCameraOpenTime()));
                                break;
                            case ACTION_GET_HISTOGRAM:
                                setResultData(mHistogram.getHistogramString());
                                break;
                            case ACTION_GET_NUM_FRAMES:
                                setResultData(Long.toString(mHistogram.getNumFrames()));
                                break;
                            case ACTION_GET_NUM_DROPPED_FRAMES:
                                setResultData(Long.toString(mHistogram.getNumDroppedFrames()));
                                break;
                            case ACTION_GET_PREVIEW_RESOLUTIONS:
                                setResultData(mCameraFragment.getPreviewResolutions());
                                break;
                            case ACTION_GET_PREVIEW_SIZE:
                                setResultData(mCameraFragment.getPreviewSize());
                                break;
                            case ACTION_GET_RECORDING_RESOLUTIONS:
                                setResultData(mCameraFragment.getRecordingResolutions());
                                break;
                            case ACTION_GET_RECORDING_SIZE:
                                setResultData(mCameraFragment.getRecordingSize());
                                break;
                            case ACTION_RESET_HISTOGRAM:
                                mHistogram.resetHistogram();
                                break;
                            case ACTION_GET_LAST_SNAPSHOT_TIME:
                                setResultData(Long.toString(mHistogram.getLastSnapshotTime()));
                                break;
                            case ACTION_GET_SNAPSHOT_RESOLUTIONS:
                                setResultData(mCameraFragment.getSnapshotResolutions());
                                break;
                            case ACTION_GET_SNAPSHOT_SIZE:
                                setResultData(mCameraFragment.getSnapshotSize());
                                break;
                            case ACTION_RESET_CAMERA:
                                mCameraFragment.onPause();
                                mCameraFragment.onResume();
                                mHistogram.resetHistogram();
                                break;
                            case ACTION_SET_TARGET_FPS:
                                int fps = intent.getIntExtra(KEY_FPS, DEFAULT_FPS);
                                mHistogram.setTargetFrameDuration((int) (1000.0 / fps));
                                mCameraFragment.setTargetFps(fps);
                                mCameraFragment.startPreview();
                                break;
                            case ACTION_SET_TARGET_RESOLUTION:
                                int width = intent.getIntExtra(KEY_WIDTH, DEFAULT_WIDTH);
                                int height = intent.getIntExtra(KEY_HEIGHT, DEFAULT_HEIGHT);
                                if (width == DEFAULT_WIDTH && height == DEFAULT_HEIGHT) {
                                    mCameraFragment.setTargetResolution(null);
                                } else {
                                    mCameraFragment.setTargetResolution(new Size(width, height));
                                }
                                mCameraFragment.startPreview();
                                break;
                            case ACTION_START_RECORDING:
                                String videoFilename = mCameraFragment.startRecordingVideo();
                                setResultData(videoFilename);
                                break;
                            case ACTION_STOP_RECORDING:
                                mCameraFragment.stopRecordingVideo();
                                break;
                            case ACTION_TAKE_PHOTO:
                                String snapshotFilename = mCameraFragment.takeCameraPicture();
                                setResultData(snapshotFilename);
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
        filter.addAction(ACTION_GET_AVG_SNAPSHOT_TIME);
        filter.addAction(ACTION_GET_CAMERA_CLOSE_TIME);
        filter.addAction(ACTION_GET_CAMERA_OPEN_TIME);
        filter.addAction(ACTION_GET_HISTOGRAM);
        filter.addAction(ACTION_GET_LAST_SNAPSHOT_TIME);
        filter.addAction(ACTION_GET_NUM_FRAMES);
        filter.addAction(ACTION_GET_NUM_DROPPED_FRAMES);
        filter.addAction(ACTION_GET_PREVIEW_RESOLUTIONS);
        filter.addAction(ACTION_GET_PREVIEW_SIZE);
        filter.addAction(ACTION_GET_RECORDING_RESOLUTIONS);
        filter.addAction(ACTION_GET_RECORDING_SIZE);
        filter.addAction(ACTION_RESET_CAMERA);
        filter.addAction(ACTION_RESET_HISTOGRAM);
        filter.addAction(ACTION_SET_TARGET_FPS);
        filter.addAction(ACTION_SET_TARGET_RESOLUTION);
        filter.addAction(ACTION_GET_SNAPSHOT_RESOLUTIONS);
        filter.addAction(ACTION_GET_SNAPSHOT_SIZE);
        filter.addAction(ACTION_START_RECORDING);
        filter.addAction(ACTION_STOP_RECORDING);
        filter.addAction(ACTION_TAKE_PHOTO);
        return filter;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        mCameraFragment = new Camera2VideoFragment();
        setContentView(R.layout.activity_camera);
        getFragmentManager().beginTransaction().replace(R.id.container, mCameraFragment).commit();
        this.registerReceiver(mReceiver, getFilter());
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        this.unregisterReceiver(mReceiver);
    }

    public CaptureCallbackHistogram getHistogram() {
        return mHistogram;
    }
}
