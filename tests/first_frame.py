#!/usr/bin/env python3
"""First-frame probe: bytes and latency until the first complete frame.

usage: first_frame.py <argv...>   (env: winsize 50x160 @ 3200x1800 px)
Scans incrementally (pdfpane's lesson: a whole-buffer rescan per read is
quadratic and inflates multi-MB frames by hundreds of ms).
"""
import fcntl, os, pty, select, struct, subprocess, sys, termios, time

END = b"\x1b[?2026l"
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 160, 3200, 1800))
env = dict(os.environ, MANGAP_KITTY_GRAPHICS="1", PDFPANE_KITTY_GRAPHICS="1")
env.setdefault("MANGAP_CONFIG_PATH", os.path.join(os.environ.get("TMPDIR", "/tmp"), "mangap-bench-config"))
t0 = time.time()
p = subprocess.Popen(sys.argv[1:], stdin=slave, stdout=slave, stderr=subprocess.DEVNULL, env=env)
buf, scanned, first_byte = bytearray(), 0, None
while time.time() - t0 < 10:
    r, _, _ = select.select([master], [], [], 0.005)
    if r:
        buf.extend(os.read(master, 1 << 20))
        if first_byte is None:
            first_byte = time.time() - t0
    if buf.find(END, max(0, scanned - len(END))) != -1:
        break
    scanned = len(buf)
t = time.time() - t0
print(f"first_frame_ms={t*1e3:.0f} first_byte_ms={(first_byte or 0)*1e3:.0f} bytes={len(buf)}")
os.write(master, b"q")
time.sleep(0.2)
p.kill()
