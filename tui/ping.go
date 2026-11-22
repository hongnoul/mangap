package tui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/codewandler/spinners"
	"github.com/hongnoul/mangap/network"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/style"
	"github.com/muesli/reflow/truncate"
)

const (
	pingTimeout = 6 * time.Second
	pingBarW    = 12
	pingOK      = "#34d399" // aurora green
	pingSlow    = "#fbbf24"
	pingDown    = "#f87171"
	pingSlowAt  = 1500 * time.Millisecond
)

type pingState int

const (
	pingPending pingState = iota
	pingUp
	pingFailed
	pingLocal // custom source with no URL to ping
)

type pingRow struct {
	name    string
	state   pingState
	latency time.Duration
	detail  string
}

// pinger checks every enabled source on the home screen, showing an
// Aurora shimmer per row until its answer lands.
type pinger struct {
	rows []pingRow
	bar  loader
	gen  int
}

type pingResultMsg struct {
	gen     int
	idx     int
	ok      bool
	latency time.Duration
	detail  string
}

func newPinger() pinger {
	return pinger{bar: newLoaderWith(spinners.Aurora)}
}

// start (re)pings every enabled source. Stale answers from an earlier run
// are dropped by gen.
func (p *pinger) start() tea.Cmd {
	p.gen++
	p.rows = nil
	var cmds []tea.Cmd
	for _, prov := range provider.Ordered() {
		row := pingRow{name: prov.Name}
		if prov.PingURL == "" {
			row.state = pingLocal
		} else {
			cmds = append(cmds, pingCmd(p.gen, len(p.rows), prov.PingURL))
		}
		p.rows = append(p.rows, row)
	}
	if len(cmds) == 0 {
		p.bar.stop()
		return nil
	}
	return tea.Batch(append(cmds, p.bar.start())...)
}

func pingCmd(gen, idx int, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()
		res := pingResultMsg{gen: gen, idx: idx}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			res.detail = "bad url"
			return res
		}
		req.Header.Set("User-Agent", "mangap")
		t0 := time.Now()
		resp, err := network.Client.Do(req)
		res.latency = time.Since(t0)
		if err != nil {
			if ctx.Err() != nil {
				res.detail = "timeout"
			} else {
				res.detail = "unreachable"
			}
			return res
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
		// Any answer below 500 means the host is up (Mangapill 405s HEAD-ish
		// probes but serves pages fine).
		res.ok = resp.StatusCode < 500
		if !res.ok {
			res.detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return res
	}
}

// update stores a result and stops the shimmer once every row settled.
func (p *pinger) update(msg pingResultMsg) {
	if msg.gen != p.gen || msg.idx >= len(p.rows) {
		return
	}
	r := &p.rows[msg.idx]
	r.latency = msg.latency
	r.detail = msg.detail
	if msg.ok {
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

// view renders one line per source:
//
//	Mangapill  ░▒▓█▓▒░░▒▓█▓  checking
//	Mangadex   ●  online     214ms
func (p *pinger) view(maxW int) string {
	if len(p.rows) == 0 {
		return ""
	}
	nameW := 0
	for _, r := range p.rows {
		nameW = imax(nameW, len([]rune(r.name)))
	}
	dot := func(c string) string {
		return style.Paint([]string{"●"}, func(int, int, rune) string { return c })
	}
	var lines []string
	for i, r := range p.rows {
		name := fmt.Sprintf("%-*s", nameW, r.name)
		var status string
		switch r.state {
		case pingPending:
			status = p.bar.viewAt(pingBarW, i*3) + "  " + style.Faint("checking")
		case pingUp:
			c, word := pingOK, "online"
			if r.latency >= pingSlowAt {
				c, word = pingSlow, "slow"
			}
			status = fmt.Sprintf("%s %-*s%s", dot(c), pingBarW-1, word,
				style.Faint(fmt.Sprintf("%5dms", r.latency.Milliseconds())))
		case pingFailed:
			status = fmt.Sprintf("%s %-*s%s", dot(pingDown), pingBarW-1, "offline", style.Faint(r.detail))
		case pingLocal:
			status = style.Faint(fmt.Sprintf("○ %-*s", pingBarW-1, "custom"))
		}
		line := style.Bold(name) + "  " + status
		lines = append(lines, truncate.String(line, uint(imax(maxW, 1))))
	}
	return strings.Join(lines, "\n")
}
