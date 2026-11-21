// Package input parses reader keys from raw terminal bytes. Batching is
// the caller's job (read everything available, parse it all, draw once),
// which caps queueing latency at one frame under key autorepeat.
package input

// Action is one reader command.
type Action int

const (
	None Action = iota
	Quit
	// Next and Prev move one sheet in reading order.
	Next
	Prev
	// Left and Right are physical directions; the viewer maps them to
	// Next/Prev by reading direction (RTL: left = next).
	Left
	Right
	First
	Last
	RotateCW
	RotateCCW
	Tone
	Spread
	Direction
	Goto
	Digit
	Backspace
	Enter
)

// Key is a parsed key: an action, plus the digit for Digit.
type Key struct {
	Action Action
	Digit  int
}

// Parser turns a byte stream into keys. It keeps a trailing incomplete
// escape sequence pending until the next Feed completes it, so an arrow
// key split across two reads stays one key.
type Parser struct {
	buf []byte
}

// Feed appends bytes and returns every complete key in them.
func (p *Parser) Feed(b []byte) []Key {
	p.buf = append(p.buf, b...)
	var keys []Key
	for len(p.buf) > 0 {
		k, n := parse(p.buf)
		if n == 0 {
			break // incomplete escape: wait for more bytes
		}
		p.buf = p.buf[n:]
		if k.Action != None {
			keys = append(keys, k)
		}
	}
	if len(p.buf) == 0 {
		p.buf = p.buf[:0]
	}
	return keys
}

// Pending reports whether an incomplete sequence is buffered.
func (p *Parser) Pending() bool { return len(p.buf) > 0 }

// Flush resolves a lone pending ESC as a bare escape (no-op), used when
// no continuation arrives.
func (p *Parser) Flush() { p.buf = p.buf[:0] }

// parse returns the first key in b and how many bytes it consumed;
// n == 0 means b holds only the start of an escape sequence.
func parse(b []byte) (Key, int) {
	c := b[0]
	if c != 0x1b {
		return single(c), 1
	}
	if len(b) < 2 {
		return Key{}, 0
	}
	switch b[1] {
	case '[':
		// CSI: parameters then one final byte in 0x40..0x7e.
		for i := 2; i < len(b); i++ {
			if b[i] >= 0x40 && b[i] <= 0x7e {
				return csi(b[2:i], b[i]), i + 1
			}
		}
		return Key{}, 0
	case 'O':
		// SS3: application-mode arrows/home/end.
		if len(b) < 3 {
			return Key{}, 0
		}
		return csi(nil, b[2]), 3
	}
	// ESC + anything else (alt-key): consumed, unbound.
	return Key{}, 2
}

func csi(params []byte, final byte) Key {
	switch final {
	case 'A':
		return Key{Action: Prev}
	case 'B':
		return Key{Action: Next}
	case 'C':
		return Key{Action: Right}
	case 'D':
		return Key{Action: Left}
	case 'H':
		return Key{Action: First}
	case 'F':
		return Key{Action: Last}
	case '~':
		switch string(params) {
		case "5": // PageUp
			return Key{Action: Prev}
		case "6": // PageDown
			return Key{Action: Next}
		case "1", "7":
			return Key{Action: First}
		case "4", "8":
			return Key{Action: Last}
		}
	}
	return Key{}
}

func single(c byte) Key {
	switch c {
	case 'q', 3: // q, Ctrl-C
		return Key{Action: Quit}
	case 'j', ' ', 'n':
		return Key{Action: Next}
	case 'k', 'p':
		return Key{Action: Prev}
	case 'h':
		return Key{Action: Left}
	case 'l':
		return Key{Action: Right}
	case 'g':
		return Key{Action: First}
	case 'G':
		return Key{Action: Last}
	case 'r':
		return Key{Action: RotateCW}
	case 'R':
		return Key{Action: RotateCCW}
	case 'd':
		return Key{Action: Tone}
	case 's':
		return Key{Action: Spread}
	case 'D':
		return Key{Action: Direction}
	case 'f':
		return Key{Action: Goto}
	case '\r', '\n':
		return Key{Action: Enter}
	case 0x7f, 0x08:
		return Key{Action: Backspace}
	}
	if c >= '0' && c <= '9' {
		return Key{Action: Digit, Digit: int(c - '0')}
	}
	return Key{}
}
