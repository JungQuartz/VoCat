#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
USB Interface 3 (Quectel AT Bulk Endpoint) to Virtual PTY Serial Bridge
"""
import os
import pty
import sys
import time
import threading
import usb.core
import usb.util

VID = 0x2C7C
PID = 0x6005
INTF_NUM = 3
LINK_PATH = "/tmp/ttyQuectelModem"

def main():
    while True:
        try:
            dev = usb.core.find(idVendor=VID, idProduct=PID)
            if not dev:
                time.sleep(2)
                continue
            
            cfg = dev.get_active_configuration()
            intf = cfg[(INTF_NUM, 0)]
            ep_out = usb.util.find_descriptor(intf, custom_match=lambda e: (e.bEndpointAddress & 0x80 == 0) and (e.bmAttributes & 0x03 == 2))
            ep_in = usb.util.find_descriptor(intf, custom_match=lambda e: (e.bEndpointAddress & 0x80 != 0) and (e.bmAttributes & 0x03 == 2))

            if not ep_out or not ep_in:
                time.sleep(2)
                continue

            master, slave = pty.openpty()
            slave_name = os.ttyname(slave)
            try:
                if os.path.exists(LINK_PATH) or os.path.islink(LINK_PATH):
                    os.unlink(LINK_PATH)
                os.symlink(slave_name, LINK_PATH)
            except Exception as e:
                print(f"Link error: {e}")

            print(f"[PTY Bridge] Connected to Quectel Modem. Virtual Serial Port: {LINK_PATH} -> {slave_name}", flush=True)

            running = [True]

            def forward_usb_to_pty():
                while running[0]:
                    try:
                        data = ep_in.read(ep_in.wMaxPacketSize, timeout=1000)
                        if data:
                            os.write(master, bytes(data))
                    except usb.core.USBTimeoutError:
                        continue
                    except Exception as e:
                        running[0] = False
                        break

            t = threading.Thread(target=forward_usb_to_pty, daemon=True)
            t.start()

            while running[0]:
                try:
                    data = os.read(master, 1024)
                    if data:
                        ep_out.write(data)
                except Exception:
                    running[0] = False
                    break

            try:
                os.close(master)
                os.close(slave)
            except: pass

        except Exception as e:
            time.sleep(2)

if __name__ == '__main__':
    main()
