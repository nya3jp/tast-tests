// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import static org.chromium.arc.testapp.arcaudiotest.Constant.TAG;

import android.util.Log;

public class SamplePlayerThread extends Thread {

    long mDurationMillis;
    private SamplePlayerBytes mPlayer;

    SamplePlayerThread(SamplePlayerBytes player, long duration_millis) {
        this.mDurationMillis = duration_millis;
        this.mPlayer = player;
    }

    public void stopPlayBack() {
        mPlayer.setStop(true);
    }

    @Override
    public void run() {
        try {
            Log.i(TAG, String.format("Start playback for: %d seconds", mDurationMillis / 1000));
            mPlayer.play(mDurationMillis);
        } catch (Exception e) {
            Thread t = Thread.currentThread();
            t.getUncaughtExceptionHandler().uncaughtException(t, e);
        }
    }
}
