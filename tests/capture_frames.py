#!/usr/bin/env python3
"""Capture what mangaz view actually transmits, for eyeballing and checks.

Runs the viewer in a PTY, presses the given keys, and writes every image
transferred to OUTDIR/frame-N.png (decoded from the kitty APC chunks) plus
the stderr trace. Exits nonzero if the stream breaks the gwae contract
(continuation chunks with extra keys, chunks over 4096 bytes).

usage: capture_frames.py <mangaz> <chapter> <outdir> [keys] [--winsize R,C,PX,PY]
"""
import base64, fcntl, os, pty, re, select, struct, subprocess, sys, termios, time

BIN, DOC, OUT = sys.argv[1], sys.argv[2], sys.argv[3]
KEYS = sys.argv[4] if len(sys.argv) > 4 and not sys.argv[4].startswith("--") else ""
ws = "50,160,3200,1800"
if "--winsize" in sys.argv:
    ws = sys.argv[sys.argv.index("--winsize") + 1]
rows, cols, px, py = (int(x) for x in ws.split(","))
os.makedirs(OUT, exist_ok=True)

master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, px, py))
env = dict(os.environ, MANGAZ_KITTY_GRAPHICS="1", MANGAZ_TRACE="1")
env.setdefault("MANGAZ_CONFIG_PATH", os.path.join(os.environ.get("TMPDIR", "/tmp"), "mangaz-bench-config"))
trace = open(os.path.join(OUT, "trace.txt"), "w")
p = subprocess.Popen([BIN, "view", DOC], stdin=slave, stdout=slave, stderr=trace, env=env)
buf = bytearray()


def pump(sec):
    end = time.time() + sec
    while time.time() < end:
        r, _, _ = select.select([master], [], [], 0.02)
        if r:
            try:
                buf.extend(os.read(master, 1 << 20))
            except OSError:
                return


pump(1.5)
for k in KEYS:
    os.write(master, k.encode())
    pump(0.6)
os.write(master, b"q")
pump(0.5)
p.wait(timeout=5)

# Reassemble transfers.
apc = re.compile(rb"\x1b_G([^;\x1b]*)(?:;([^\x1b]*))?\x1b\\")
frames, cur, errors = [], None, []
for m in apc.finditer(bytes(buf)):
    ctl = dict(kv.split(b"=", 1) for kv in m.group(1).split(b",") if b"=" in kv)
    payload = m.group(2) or b""
    if len(payload) > 4096:
        errors.append(f"chunk of {len(payload)} bytes > 4096")
    if ctl.get(b"a") == b"T":
        cur = {"ctl": ctl, "data": bytearray(payload)}
        if ctl.get(b"m") == b"0":
            frames.append(cur)
            cur = None
    elif cur is not None and set(ctl) <= {b"m", b"q"}:
        cur["data"].extend(payload)
        if ctl.get(b"m") == b"0":
            frames.append(cur)
            cur = None
    elif cur is not None and ctl.get(b"a") not in (b"d", b"p"):
        errors.append(f"continuation with extra keys: {m.group(1)!r}")

for i, f in enumerate(frames):
    with open(os.path.join(OUT, f"frame-{i}.png"), "wb") as fh:
        fh.write(base64.b64decode(bytes(f["data"])))
    c = f["ctl"]
    print(f"frame-{i}: id={c[b'i'].decode()} {c[b's'].decode()}x{c[b'v'].decode()}px "
          f"span={c[b'c'].decode()}x{c[b'r'].decode()} png={len(base64.b64decode(bytes(f['data'])))}B")
print(f"exit={p.returncode} bytes={len(buf)} frames={len(frames)} "
      f"deletes={len(re.findall(rb'a=d,d=I', bytes(buf)))} "
      f"restored={'yes' if bytes(buf).endswith(b'[?1049l') else 'no'}")
for e in errors:
    print("CONTRACT:", e)
sys.exit(1 if errors or p.returncode else 0)
