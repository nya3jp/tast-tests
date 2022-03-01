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
import android.util.Rational;
import android.widget.VideoView;

/** Test Activity for the PIP Video Tast Test. */
public class VideoActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.video_activity);

        final VideoView videoView = findViewById(R.id.testvideo);
        videoView.setVideoURI(Uri.parse("android.resource://" + getPackageName() + "/raw/"
                + getIntent().getStringExtra("video")));
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
}
