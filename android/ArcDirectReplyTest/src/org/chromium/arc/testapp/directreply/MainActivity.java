/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.directreply;

import android.app.Activity;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.RemoteInput;
import android.content.Context;
import android.content.Intent;
import android.graphics.drawable.Icon;
import android.view.View;
import android.os.Bundle;


public class MainActivity extends Activity {
    final String CHANNEL_ID = "org.chromium.arc.testapp.directreply";
    final String CHANNEL_NAME = "ARC Direct Reply Tester";
    final String TITLE_TEXT = "Test";
    final String BODY_TEXT = "This is a test notification";
    final String REPLY_TEXT = "Reply";
    final String PLACEHOLDER_TEXT = "Placeholder text";
    final String REPLY_KEY = "direct_reply";

    NotificationManager mNotificationManager;
    int mNotificationId = 0;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        findViewById(R.id.show_notification).setOnClickListener(v -> sendNotification());

        mNotificationManager =
                (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);

        createNotificationChannel();
    }

    private void createNotificationChannel() {
        mNotificationManager.createNotificationChannel(
                new NotificationChannel(
                        CHANNEL_ID, CHANNEL_NAME, NotificationManager.IMPORTANCE_HIGH));
    }

    private void sendNotification() {
        Icon icon = Icon.createWithResource(
                getApplicationContext(), android.R.drawable.ic_menu_send);
        PendingIntent pendingIntent =
                PendingIntent.getActivity(
                        this,
                        0,
                        new Intent(this, MainActivity.class),
                        PendingIntent.FLAG_UPDATE_CURRENT);
        RemoteInput remoteInput =
                new RemoteInput.Builder(REPLY_KEY).setLabel(PLACEHOLDER_TEXT).build();
        Notification.Action action =
                new Notification.Action.Builder(icon, REPLY_TEXT, pendingIntent)
                        .addRemoteInput(remoteInput)
                        .build();
        Notification notification =
                new Notification.Builder(this, CHANNEL_ID)
                        .setSmallIcon(android.R.drawable.ic_dialog_info)
                        .setContentTitle(TITLE_TEXT)
                        .setContentText(BODY_TEXT)
                        .addAction(action)
                        .build();
        mNotificationManager.notify(mNotificationId, notification);
        mNotificationId++;
    }
}
