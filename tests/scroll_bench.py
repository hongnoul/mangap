#!/usr/bin/env python3
"""Scroll latency benchmark: the closed feedback loop for flip speed.

Ported from pdfpane's tests/scroll_bench.py. Drives either viewer in a
PTY on the same chapter, so the two are measured by the same clock:

  scroll_bench.py --mangap ./mangap <chapter dir|cbz>
  scroll_bench.py --pdfpane pdfpane <chapter.pdf>

Boots the viewer, then measures:

1. SLOW SCROLL: single j keypresses, each waiting for the frame to land.
   Reports per-flip latency from key write to the end of the first
   complete synchronized frame (ESC[?2026l) containing page content.
   Distinguishes WARM flips (prefetch hit: placeholder-only or instant
   transfer) from COLD flips (render awaited).

2. HELD-KEY SCROLL: autorepeat burst (configurable rate/duration), then
   counts frames presented and measures time from the LAST key to the
   final settled frame (how far behind the render pipeline fell).

Usage: scroll_bench.py (--mangap|--pdfpane) <bin> <doc> [--flips N] [--rate HZ]
                       [--hold-sec S] [--winsize R,C,PX,PY] [--start-page P]
                       [--json] [--assert] [--min-hold-fps F] [--max-hold-gap-ms G]

--assert turns the report into a regression gate with deliberately
generous thresholds (the broken pipeline this guards against measured
5/45 held frames and ~500ms p50, so an order of magnitude of headroom
still separates pass from regression under machine-load noise):
  - slow-scroll p50 <= 150 ms (prefetch hit path; measured ~18 ms)
  - held-key frames >= 50% of keys (measured ~95%; baseline bug: 11%)
  - catch-up after the last key <= 2000 ms (measured ~25 ms)

--min-hold-fps F gates on frames presented WHILE the key is held, as a
rate. The per-key >=50% gate only works when the key rate is within the
pipeline's render throughput (one page per key); past it, the page-lead
clamp intentionally consumes keys without advancing a page, so the
honest freeze measure is display frames per second during the hold (the
freeze bug this guards against measured ~2 fps at 60 Hz autorepeat:
the pane sat frozen until release, then lurched; fixed measures ~20).
NOTE: fps is machine-load sensitive (measured 20 idle, 3-7 under a
concurrent cargo build), so the suite gates on --max-hold-gap-ms
instead.

--max-hold-gap-ms G gates on the LONGEST stretch without a frame during
the hold (first key to last key, plus the tail to the last key). This
is the freeze itself: the bug presents a few frames in the first 100ms
and then nothing until release (gap ~= the whole hold), while a healthy
pipeline keeps presenting even when machine load slows every render.
Both gates run WITHOUT --assert for high-rate runs: past the pipeline's
throughput the per-key >=50% frame gate cannot hold by design.
"""
import fcntl, json, os, pty, select, struct, subprocess, sys, termios, time

if sys.argv[1] in ("--mangap", "--pdfpane"):
    KIND, BIN, DOC = sys.argv[1][2:], sys.argv[2], sys.argv[3]
else:  # pdfpane's original positional form
    KIND, BIN, DOC = "pdfpane", sys.argv[1], sys.argv[2]


def arg(name, default, cast=str):
    if name in sys.argv:
        return cast(sys.argv[sys.argv.index(name) + 1])
    return default


FLIPS = arg("--flips", 12, int)
RATE = arg("--rate", 30.0, float)
HOLD_SEC = arg("--hold-sec", 2.0, float)
START = arg("--start-page", 1, int)
winsize = tuple(int(x) for x in arg("--winsize", "50,160,3200,1800").split(","))
AS_JSON = "--json" in sys.argv
DO_ASSERT = "--assert" in sys.argv
MIN_HOLD_FPS = arg("--min-hold-fps", 0.0, float)
MAX_HOLD_GAP_MS = arg("--max-hold-gap-ms", 0.0, float)

FRAME_END = b"\x1b[?2026l"

# Page count bounds how many held-scroll frames are even possible (flips
# past the end are zero-byte no-ops); 0 = unknown.
IMAGE_EXT = (".jpg", ".jpeg", ".png", ".gif", ".webp")
pages = 0
if os.path.isdir(DOC):
    pages = sum(1 for n in os.listdir(DOC) if n.lower().endswith(IMAGE_EXT))
elif DOC.lower().endswith((".cbz", ".zip")):
    import zipfile
    with zipfile.ZipFile(DOC) as z:
        pages = sum(1 for n in z.namelist() if n.lower().endswith(IMAGE_EXT))
else:
    try:
        pages = int(subprocess.run(
            ["mdls", "-name", "kMDItemNumberOfPages", "-raw", DOC],
            capture_output=True, text=True).stdout.strip())
    except (ValueError, FileNotFoundError):
        pages = 0

master, slave = pty.openpty()
rows, cols, px, py = winsize
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, px, py))
if KIND == "mangap":
    env = dict(os.environ, MANGAP_KITTY_GRAPHICS="1")
    # Isolated config so the bench never touches (or migrates) real config.
    env.setdefault("MANGAP_CONFIG_PATH", os.path.join(
        os.environ.get("TMPDIR", "/tmp"), "mangap-bench-config"))
    argv = [BIN, "view", DOC, "--page", str(START)]
else:
    env = dict(os.environ, PDFPANE_KITTY_GRAPHICS="1")
    argv = [BIN, DOC, str(START)]
p = subprocess.Popen(argv, stdin=slave, stdout=slave,
                     stderr=subprocess.DEVNULL, env=env)
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


def wait_frame(timeout):
    """Wait until a complete frame (ends with ?2026l) arrives after `mark`.
    Returns (latency_s, bytes_out, had_transfer) or None on timeout.
    Scans INCREMENTALLY (only bytes not yet searched, with an overlap for
    a split marker): a whole-buffer rescan per 1KB PTY read is quadratic
    and inflates a 1.7MB frame's measured latency by ~500ms, swamping the
    thing being measured."""
    mark = len(buf)
    scanned = mark
    t0 = time.time()
    deadline = t0 + timeout
    while time.time() < deadline:
        r, _, _ = select.select([master], [], [], 0.005)
        if r:
            try:
                buf.extend(os.read(master, 1 << 20))
            except OSError:
                break
        lo = max(mark, scanned - len(FRAME_END) + 1)
        if buf.find(FRAME_END, lo) != -1:
            t = time.time() - t0
            new = bytes(buf[mark:])
            return (t, len(new), b"a=T" in new)
        scanned = len(buf)
    return None


def settle(quiet=0.4, timeout=30.0):
    """Pump until no new output for `quiet` seconds (prefetch flushed)."""
    deadline = time.time() + timeout
    last = len(buf)
    last_change = time.time()
    while time.time() < deadline:
        pump(0.05)
        if len(buf) != last:
            last = len(buf)
            last_change = time.time()
        elif time.time() - last_change > quiet:
            return


# --- boot ---
t_boot0 = time.time()
got = wait_frame(60)
assert got, "no boot frame"
boot_s = got[0]
settle(quiet=1.0)  # let prefetch warm neighbors

# --- slow scroll ---
slow = []
for i in range(FLIPS):
    os.write(master, b"j")
    f = wait_frame(30)
    assert f, f"flip {i} never produced a frame"
    lat, nbytes, xfer = f
    slow.append({"ms": lat * 1e3, "bytes": nbytes, "transfer": xfer})
    settle(quiet=0.25)  # let the prefetcher catch up between flips

# --- held-key scroll (autorepeat) ---
settle(quiet=1.0)
mark = len(buf)
n_keys = int(RATE * HOLD_SEC)
t_hold0 = time.time()
hold_frame_times = []
frames_seen = 0
for i in range(n_keys):
    os.write(master, b"j")
    pump(max(0.0, (t_hold0 + (i + 1) / RATE) - time.time()))
    c = buf.count(FRAME_END, mark)
    while frames_seen < c:
        frames_seen += 1
        hold_frame_times.append(time.time())
t_last_key = time.time()
frames_during = frames_seen
# Longest stretch without a presented frame, from the first key through
# the last key. Frame times are quantized to the pump granularity
# (~inter-key gap), fine relative to the freeze being detected (seconds).
edges = [t_hold0] + hold_frame_times + [t_last_key]
hold_gap_ms = max((b - a) for a, b in zip(edges, edges[1:])) * 1e3
# Time until the pipeline settles on the final page. A fully keeping-up
# pipeline may have presented the final frame BEFORE the last key
# returned (each key's frame landed within the inter-key gap), in which
# case no further frame arrives at all: that is 0ms of catch-up, the
# best possible result, not a timeout.
f = wait_frame(2.0)
catch_up = (time.time() - t_last_key) if f else 0.0
settle(quiet=0.8)
hold_out = bytes(buf[mark:])
frames = hold_out.count(FRAME_END)
transfers = hold_out.count(b"a=T") - hold_out.count(b"a=T,U=1,t=d,f=100,q=2,m=1")

os.write(master, b"q")
time.sleep(0.2)
p.terminate()

ms = sorted(x["ms"] for x in slow)
res = {
    "boot_ms": round(boot_s * 1e3, 1),
    "flips": len(slow),
    "flip_ms_min": round(ms[0], 1),
    "flip_ms_p50": round(ms[len(ms) // 2], 1),
    "flip_ms_p90": round(ms[int(len(ms) * 0.9)], 1),
    "flip_ms_max": round(ms[-1], 1),
    "warm_flips": sum(1 for x in slow if not x["transfer"]),
    "cold_flips": sum(1 for x in slow if x["transfer"]),
    "hold_keys": n_keys,
    "hold_frames": frames,
    "hold_frames_during": frames_during,
    "hold_fps": round(frames_during / (t_last_key - t_hold0), 1),
    "hold_max_gap_ms": round(hold_gap_ms, 1),
    "hold_catch_up_ms": round(catch_up * 1e3, 1),
    "per_flip": [round(x["ms"], 1) for x in slow],
}
if AS_JSON:
    print(json.dumps(res))
else:
    for k, v in res.items():
        print(f"{k}: {v}")

if DO_ASSERT:
    p50 = res["flip_ms_p50"]
    assert p50 <= 150, f"slow-scroll p50 {p50}ms > 150ms: prefetch hit path regressed"
    # A flip past the last page is a no-op (zero bytes), so a short
    # document bounds presentable frames by its page count, not the key
    # count. The slow-scroll phase consumed FLIPS pages past START first.
    reachable = max(pages - (START + FLIPS), 0) if pages else n_keys
    expected = min(n_keys, reachable)
    assert expected >= 10, \
        f"--assert needs >=10 held-scroll pages left ({expected}: {pages} total, start {START}, {FLIPS} flips); use a longer PDF"
    assert frames >= expected * 0.5, \
        f"held-key scroll presented {frames}/{expected} presentable frames (<50%): pipeline falling behind"
    assert catch_up * 1e3 <= 2000, \
        f"catch-up {res['hold_catch_up_ms']}ms > 2000ms after the last key"
    print("SCROLL BENCH PASS")

if MIN_HOLD_FPS > 0:
    fps = res["hold_fps"]
    assert fps >= MIN_HOLD_FPS, \
        f"held-key scroll presented {fps} fps during the hold " \
        f"(< {MIN_HOLD_FPS}): pane freezing while the key is down"
    assert catch_up * 1e3 <= 2000, \
        f"catch-up {res['hold_catch_up_ms']}ms > 2000ms after the last key"
    print("HOLD FPS GATE PASS")

if MAX_HOLD_GAP_MS > 0:
    gap = res["hold_max_gap_ms"]
    assert gap <= MAX_HOLD_GAP_MS, \
        f"held-key scroll froze for {gap}ms during the hold " \
        f"(> {MAX_HOLD_GAP_MS}ms): pane not moving while the key is down"
    assert catch_up * 1e3 <= 2000, \
        f"catch-up {res['hold_catch_up_ms']}ms > 2000ms after the last key"
    print("HOLD FREEZE GATE PASS")
