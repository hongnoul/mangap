package input

import "testing"

func actions(ks []Key) []Action {
	out := make([]Action, len(ks))
	for i, k := range ks {
		out[i] = k.Action
	}
	return out
}

func eq(a, b []Action) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSingleKeys(t *testing.T) {
	var p Parser
	got := actions(p.Feed([]byte("jjkhlqx")))
	want := []Action{Next, Next, Prev, Left, Right, Quit}
	if !eq(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestArrowsAndSplitEscape(t *testing.T) {
	var p Parser
	got := actions(p.Feed([]byte("\x1b[C\x1b[D\x1b")))
	if !eq(got, []Action{Right, Left}) || !p.Pending() {
		t.Fatalf("got %v pending=%v", got, p.Pending())
	}
	got = actions(p.Feed([]byte("[Bj")))
	if !eq(got, []Action{Next, Next}) {
		t.Fatalf("split arrow: got %v", got)
	}
}

func TestPageKeysAndSS3(t *testing.T) {
	var p Parser
	got := actions(p.Feed([]byte("\x1b[6~\x1b[5~\x1bOC\x1b[1;5D")))
	want := []Action{Next, Prev, Right, Left}
	if !eq(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDigits(t *testing.T) {
	var p Parser
	ks := p.Feed([]byte("f12\x7f"))
	if len(ks) != 4 || ks[0].Action != Goto || ks[1].Digit != 1 || ks[2].Digit != 2 || ks[3].Action != Backspace {
		t.Fatalf("got %+v", ks)
	}
}
