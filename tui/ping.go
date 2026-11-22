package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codewandler/spinners"
	"github.com/hongnoul/mangap/network"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/style"
)

const (
	pingTimeout = 8 * time.Second
	pingBarW    = 4
	pingOK      = "#34d399" // aurora green
	pingSlow    = "#fbbf24"
	pingDown    = "#f87171"
	pingSlowAt  = 300 * time.Millisecond
)

type pingState int

const (
	pingPending pingState = iota
	pingUp
	pingFailed
	pingLocal // custom source with no URL to ping
)

type pingRow struct {
	state  pingState
	rtt    time.Duration // TCP handshake round trip
	detail string
}

// pinger measures network RTT to every enabled source, starting at init.
// Results are shown inside the source selector, keyed by source name.
type pinger struct {
	rows  map[string]*pingRow
	names []string // ping order, for shimmer offsets
	bar   loader
	gen   int
}

type pingResultMsg struct {
	gen    int
	name   string
	rtt    time.Duration
	detail string
}

func newPinger() pinger {
	return pinger{bar: newLoaderWith(spinners.Aurora), rows: map[string]*pingRow{}}
}

// start (re)pings every enabled source. Stale answers from an earlier run
// are dropped by gen.
func (p *pinger) start() tea.Cmd {
	p.gen++
	p.rows = map[string]*pingRow{}
	p.names = nil
	var cmds []tea.Cmd
	for _, prov := range provider.Ordered() {
		row := &pingRow{}
		if prov.PingURL == "" {
			row.state = pingLocal
		} else {
			cmds = append(cmds, pingCmd(p.gen, prov.Name, prov.PingURL))
		}
		p.rows[prov.Name] = row
		p.names = append(p.names, prov.Name)
	}
	if len(cmds) == 0 {
		p.bar.stop()
		return nil
	}
	return tea.Batch(append(cmds, p.bar.start())...)
}

func pingCmd(gen int, name, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()
		rtt, detail := network.PingRTT(ctx, url)
		return pingResultMsg{gen: gen, name: name, rtt: rtt, detail: detail}
	}
}

// update stores a result and stops the shimmer once every row settled.
func (p *pinger) update(msg pingResultMsg) {
	r, ok := p.rows[msg.name]
	if msg.gen != p.gen || !ok {
		return
	}
	r.rtt, r.detail = msg.rtt, msg.detail
	if msg.detail == "" {
		r.state = pingUp
	} else {
		r.state = pingFailed
	}
	if !p.pending() {
		p.bar.stop()
	}
}

func (p *pinger) pending() bool {
	for _, r := range p.rows {
		if r.state == pingPending {
			return true
		}
	}
	return false
}

// badge is the compact status for one source: "● 27ms", a shimmer while
// checking, "● timeout" when down, or "" when unknown (disabled source).
func (p *pinger) badge(name string) string {
	r, ok := p.rows[name]
	if !ok {
		return ""
	}
	dot := func(c string) string {
		return style.Paint([]string{"●"}, func(int, int, rune) string { return c })
	}
	switch r.state {
	case pingPending:
		off := 0
		for i, n := range p.names {
			if n == name {
				off = i * 3
			}
		}
		return p.bar.viewAt(pingBarW, off)
	case pingUp:
		c := pingOK
		if r.rtt >= pingSlowAt {
			c = pingSlow
		}
		return dot(c) + style.Faint(fmt.Sprintf(" %dms", r.rtt.Milliseconds()))
	case pingFailed:
		return dot(pingDown) + style.Faint(" "+r.detail)
	case pingLocal:
		return style.Faint("○ local")
	}
	return ""
}
