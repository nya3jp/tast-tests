// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import static org.chromium.arc.testapp.arcaudiotest.Constant.TAG;

import android.content.Intent;
import android.content.res.AssetFileDescriptor;
import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioTrack;
import android.os.Bundle;
import android.util.Log;

/*
 * Activity for Chrome OS ARC++/ARCVM audio power performance tast.
 */
public class PlaybackPerformanceActivity extends MainActivity {

    private static final String KEY_PERFORMANCE_MODE = "perf_mode";
    private static final String KEY_DURATION = "duration";
    private static final int DEFAULT_DURATION_SECOND = 60;
    private int mPerformanceMode = AudioTrack.PERFORMANCE_MODE_NONE;
    private SamplePlayerThread mThread;

    private int mDurationSecond = DEFAULT_DURATION_SECOND;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        // Get the message from the intent
        Intent intent = getIntent();
        mPerformanceMode = intent
            .getIntExtra(KEY_PERFORMANCE_MODE, AudioTrack.PERFORMANCE_MODE_NONE);
        Log.i(TAG, String.format("Set PerformanceMode: %d", mPerformanceMode));

        mDurationSecond = intent
            .getIntExtra(KEY_DURATION, DEFAULT_DURATION_SECOND);
        Log.i(TAG, String.format("Set mDurationSecond: %d", mDurationSecond));
    }

    @Override
    protected void onResume() {
        super.onResume();
        try {
            AssetFileDescriptor fd =
                getApplicationContext().getResources().openRawResourceFd(R.raw.sinesweepraw);
            SamplePlayerBytes player =
                new SamplePlayerBytes(
                    44100,
                    AudioFormat.ENCODING_PCM_16BIT,
                    AudioFormat.CHANNEL_OUT_STEREO,
                    fd);
            player.setPerformanceMode(mPerformanceMode);
            // Using any usage other than USAGE_MEDIA to prevent the performance mode from being
            // automatically change from PERFORMANCE_MODE_NONE to PERFORMANCE_MODE_POWER_SAVING
            // by AudioTrack.
            player.setAudioAttributes(new AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_GAME)
                .build());
            mThread = new SamplePlayerThread(player, mDurationSecond * 1000);
            mThread.start();
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    protected void onDestroy() {
        super.onDestroy();
        try {
            mThread.stopPlayBack();
            mThread.join();
        } catch (InterruptedException e) {
            e.printStackTrace();
        }
    }

}
