/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.pictureinpicturevideo;

import android.app.Activity;
import android.app.PictureInPictureParams;
import android.media.MediaPlayer;
import android.media.MediaPlayer.OnPreparedListener;
import android.net.Uri;
import android.os.Bundle;
import android.R.id;
import android.util.DisplayMetrics;
import android.util.Rational;
import android.view.KeyEvent;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.RelativeLayout;
import android.widget.VideoView;

/** Test Activity for the PIP Video Tast Test. */
public class VideoActivity extends Activity {

    private static boolean wantRedSquare = false;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.video_activity);

        if (wantRedSquare) {
            final int fiftyDp = (int) (50.f * getResources().getDisplayMetrics().density + 0.5f);
            RelativeLayout.LayoutParams redSquareLayoutParams =
                new RelativeLayout.LayoutParams(fiftyDp, fiftyDp);
            redSquareLayoutParams.addRule(RelativeLayout.CENTER_IN_PARENT);

            View redSquare = new View(this);
            redSquare.setLayoutParams(redSquareLayoutParams);
            redSquare.setBackgroundColor(0xffff0000);
            ((ViewGroup) findViewById(R.id.testlayout)).addView(redSquare);
        }

        final VideoView videoView = findViewById(R.id.testvideo);
        videoView.setVideoURI(Uri.parse(
            "android.resource://" + getPackageName() + "/raw/bear-320x240.h264"));
        videoView.setOnPreparedListener(new OnPreparedListener() {
          @Override
          public void onPrepared(MediaPlayer mp) {
            mp.setVolume(0, 0);
            mp.setLooping(true);
          }
        });
        videoView.start();
    }

    @Override
    protected void onUserLeaveHint() {
        super.onUserLeaveHint();

        final VideoView videoView = findViewById(R.id.testvideo);
        enterPictureInPictureMode(
            new PictureInPictureParams.Builder()
                .setAspectRatio(new Rational(videoView.getWidth(), videoView.getHeight()))
                .build());
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        if (event.getMetaState() == 0x0 && event.getKeyCode() == KeyEvent.KEYCODE_SPACE) {
            wantRedSquare = true;
            return true;
        }

        return super.dispatchKeyEvent(event);
    }
}
