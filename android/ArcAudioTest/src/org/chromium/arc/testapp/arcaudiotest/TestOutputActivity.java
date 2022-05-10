// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.content.res.AssetFileDescriptor;
import android.media.AudioFormat;
import android.os.Bundle;
import android.util.Log;

/**
 * Activity for ChromeOS ARC++/ARCVM audio validity tast.
 */
public class TestOutputActivity extends MainActivity {

    public void testPlaySineSweepBytes() {
        try {
            AssetFileDescriptor fd =
                getApplicationContext().getResources().openRawResourceFd(R.raw.sinesweepraw);
            SamplePlayerBytes player =
                new SamplePlayerBytes(
                    44100,
                    AudioFormat.ENCODING_PCM_16BIT,
                    AudioFormat.CHANNEL_OUT_STEREO,
                    fd);
            Log.d(Constant.TAG, "start playing sound");
            player.play(5000);
            Log.d(Constant.TAG, "finish playing sound");
            markAsPassed();
        } catch (Exception e) {
            markAsFailed("testPlaySineSweepBytes failed. " + e.toString());
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
    }

    @Override
    protected void onStart() {
        super.onStart();
        Log.i(Constant.TAG, "start testPlaySineSweepBytes");
        testPlaySineSweepBytes();
        Log.i(Constant.TAG, "finish testPlaySineSweepBytes");
    }
}
