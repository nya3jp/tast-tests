/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.multicast_forwarder;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.CheckBox;
import android.widget.EditText;

import java.io.IOException;
import java.net.DatagramPacket;
import java.net.InetAddress;
import java.net.MulticastSocket;
import java.net.NetworkInterface;
import java.nio.ByteBuffer;
import java.nio.charset.Charset;
import java.util.Enumeration;

/**
 * Test Activity for the arcapp.MulticastForwarder Tast test.
 *
 * <p>This has a two buttons that sends mDNS and SSDP packet respectively. The parameter for the
 * packet is configured by two EditText (hostname and port).
 *
 * <p>There is also a CheckBox to toggle between IPv4 and IPv6.
 */
public class MulticastForwarderActivity extends Activity {
    private final int MdnsPort = 5353;
    private final int SsdpPort = 1900;
    private EditText mData;
    private EditText mPort;
    private CheckBox mIPv6;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mData = findViewById(R.id.data);
        mPort = findViewById(R.id.port);
        mIPv6 = findViewById(R.id.checkbox_ipv6);
    }

    /**
     * Sends mDNS multicast packet.
     *
     * <p>This is called as an onClick handler.
     *
     * @param view {@link View} that was clicked.
     */
    public void testMdns(View view) {
        try {
            // Create multicast socket and join group.
            // https://www.iana.org/assignments/multicast-addresses/multicast-addresses.xhtml
            // https://www.iana.org/assignments/ipv6-multicast-addresses/ipv6-multicast-addresses.xhtml
            String hostname = mData.getText().toString();
            InetAddress group;
            if (mIPv6.isChecked()) {
                group = InetAddress.getByName("ff02::fb");
            } else {
                group = InetAddress.getByName("224.0.0.251");
            }

            ByteBuffer buffer = ByteBuffer.allocate(9000 /* MAX_MDNS_SIZE */);

            // Create an mDNS message.
            // https://tools.ietf.org/html/rfc6762#section-18
            buffer.putShort((short) 0x0); // Transaction ID = 0
            buffer.putShort((short) 0x0); // Flags = 0
            buffer.putShort((short) 0x1); // Number of questions = 1
            buffer.putShort((short) 0x0); // Number of answers = 0
            buffer.putInt(0x0); // Number of resource records = 0
            // Add hostname to buffer.
            for (String data : hostname.split("\\.")) {
                buffer.put((byte) data.length()); // Add string length.
                buffer.put(data.getBytes()); // Add string data.
            }
            buffer.put((byte) 0x0); // Terminator
            buffer.putShort((short) 0x1); // QTYPE = A record
            buffer.putShort((short) 0x1); // QCLASS = IN class

            testMulticast(buffer.array(), buffer.position(), group, MdnsPort);
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    /**
     * Sends SSDP multicast packet.
     *
     * <p>This is called as an onClick handler.
     *
     * @param view {@link View} that was clicked.
     */
    public void testSsdp(View view) {
        try {
            // Create multicast socket and join group.
            // https://www.iana.org/assignments/multicast-addresses/multicast-addresses.xhtml
            // https://www.iana.org/assignments/ipv6-multicast-addresses/ipv6-multicast-addresses.xhtml
            String userAgent = mData.getText().toString();
            InetAddress group;
            if (mIPv6.isChecked()) {
                group = InetAddress.getByName("ff02::c");
            } else {
                group = InetAddress.getByName("239.255.255.250");
            }

            // Create an SSDP message.
            // https://tools.ietf.org/html/draft-cai-ssdp-v1-03#section-4
            StringBuilder sb = new StringBuilder("M-SEARCH * HTTP/1.1\r\n");
            if (mIPv6.isChecked()) {
                sb.append("HOST: [FF02::C]:1900\r\n");
            } else {
                sb.append("HOST: 239.255.255.250:1900\r\n");
            }
            sb.append("MAN: \"ssdp:discover\"\r\n");
            sb.append("MX: 3\r\n");
            sb.append("ST: ssdp:all\r\n");
            sb.append("USER-AGENT: " + userAgent + "\r\n\r\n");

            byte[] buffer = sb.toString().getBytes(Charset.forName("UTF-8"));
            testMulticast(buffer, buffer.length, group, SsdpPort);

        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    private void testMulticast(byte[] payload, int len, InetAddress group, int destPort)
            throws IOException {
        MulticastSocket socket = new MulticastSocket(Integer.parseInt(mPort.getText().toString()));
        socket.joinGroup(group);

        Thread thread =
                new Thread(
                        new Runnable() {
                            @Override
                            public void run() {
                                try {
                                    // Send the packet.
                                    DatagramPacket packet =
                                            new DatagramPacket(payload, len, group, destPort);
                                    packet.setAddress(group);

                                    Enumeration<NetworkInterface> networkInterfaces =
                                            NetworkInterface.getNetworkInterfaces();
                                    while (networkInterfaces.hasMoreElements()) {
                                        NetworkInterface iface = networkInterfaces.nextElement();
                                        if (!iface.supportsMulticast()) continue;
                                        try {
                                            socket.setNetworkInterface(iface);
                                            socket.send(packet);
                                        } catch (IOException e) {
                                            e.printStackTrace();
                                        }
                                    }
                                } catch (Exception e) {
                                    e.printStackTrace();
                                }
                            }
                        });
        thread.start();
    }
}
