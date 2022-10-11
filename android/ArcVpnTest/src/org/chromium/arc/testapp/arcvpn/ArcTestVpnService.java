/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcvpn;

import android.R;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Intent;
import android.net.VpnService;
import android.os.Handler;
import android.os.HandlerThread;
import android.os.ParcelFileDescriptor;
import android.util.Log;

/**
 * Test app that starts a simple VPN. It's not expected to actually forward data in/out, just to
 * register some VPN with the system.
 *
 * To preauthorize the package and bypass user dialog:
 *   $ adb shell dumpsys wifi authorize-vpn org.chromium.arc.testapp.arcvpn
 *
 * To start the activity which then starts the service:
 *   $ adb shell am start \
 *       org.chromium.arc.testapp.arcvpn/org.chromium.arc.testapp.arcvpn.MainActivity
 */
public class ArcTestVpnService extends VpnService {
    private static final String TAG = ArcTestVpnService.class.getSimpleName();

    // Metadata for the notification.
    private static final int NOTIFICATION_ID = 1;
    private static final String NOTIFICATION_CHANNEL_ID = TAG;

    @Override
    public void onCreate() {
        super.onCreate();

        showNotification();
        ParcelFileDescriptor tunFd = setUpVpnService();

        // Execute on a separate thread so that the service doesn't look like it's ANR and killed
        // by the system.
        HandlerThread thread = new HandlerThread(TAG + " Handler");
        thread.start();
        Handler handler = new Handler(thread.getLooper());
        handler.post(() -> infiniteLoop(tunFd));
    }

    /**
     * Creates the notification to be constantly shown while the service is running. This is needed
     * to be considered a proper foreground service.
     */
    private void showNotification() {
        getSystemService(NotificationManager.class)
                .createNotificationChannel(
                        new NotificationChannel(
                                NOTIFICATION_CHANNEL_ID,
                                NOTIFICATION_CHANNEL_ID,
                                NotificationManager.IMPORTANCE_NONE));

        startForeground(
                NOTIFICATION_ID,
                new Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
                        .setSmallIcon(R.drawable.ic_dialog_info)
                        .build());
    }

    /** Registers ourselves as an actual VpnService and sets up the underlying interface. */
    private ParcelFileDescriptor setUpVpnService() {
        Intent intent = VpnService.prepare(getApplicationContext());

        return new VpnService.Builder()
                .addAddress("192.168.2.2", 24)
                .addRoute("0.0.0.0", 0)
                .establish();
    }

    /**
     * Infinitely loops and references {@code fd} to prevent the file descriptor from closing,
     * which would cause the tun interface to close as well.
     *
     * Must be called on handler thread.
     */
    private void infiniteLoop(ParcelFileDescriptor fd) {
        while (fd.getFileDescriptor().valid()) {
            // Do nothing
        }
        Log.e(TAG, "tun fd unexpectedly closed");
    }
}
