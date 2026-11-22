package tui

import (
	"context"
	"errors"

	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hongnoul/mangap/catalog"
	"github.com/hongnoul/mangap/history"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/provider/mangadex"
	"github.com/hongnoul/mangap/query"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
)

// Live search. Every keystroke repaints from local data in the same frame:
//
//  1. an exact hit in the in-memory or on-disk query cache, else
//  2. the current results narrowed to the longer query, plus matches from
//     the local catalog of every manga seen before.
//
// The network runs alongside: the first key of a burst fires at once, later
// keys wait for a short pause sized to the typing speed. Every source is
// queried in parallel and streams in on its own. Requests for a query
// that is a prefix of the current text keep running (their results are
// narrowed and shown while the exact answer is on its way), the rest are
// cancelled. Results merge into the list so rows that stay keep their
// place, cover and highlight.

const (
	// liveMinPause / liveMaxPause bound the wait after a keystroke before
	// the trailing request. The actual pause follows typing speed.
	liveMinPause = 60 * time.Millisecond
	liveMaxPause = 180 * time.Millisecond
	// liveBurstGap: a key after this much quiet starts a new burst and
	// fires immediately.
	liveBurstGap = 300 * time.Millisecond
	// liveMaxWait: during a long burst a request still goes out at least
	// this often, so fast typists see network results mid-word.
	liveMaxWait = 350 * time.Millisecond
	// liveLocalLimit caps catalog matches mixed into provisional results.
	liveLocalLimit = 30
	// liveMaxInflight bounds concurrent requests per keystroke burst.
	liveMaxInflight = 4
	// liveWarmEvery keeps the source connections from idling out.
	liveWarmEvery = 45 * time.Second
	// livePrefetchDelay is how long a highlighted result must stay
	// highlighted before its chapter list is fetched in the background.
	livePrefetchDelay = 250 * time.Millisecond
	// liveCoverPrefetch is how many covers of a speculative result are
	// fetched ahead (one screen).
	liveCoverPrefetch = singleColRows
)

// prefetchCovers downloads the first n covers into the thumbnail disk
// cache without rendering them.
func prefetchCovers(mangas []*source.Manga, n int) {
	for i, m := range mangas {
		if i >= n {
			return
		}
		u, ref := coverURL(m), m.URL
		if u == "" {
			continue
		}
		_, _ = fetchSource(u, func() (string, string, error) { return u, ref, nil })
	}
}

// liveCatalog is the local index used by live search (swapped in tests).
var liveCatalog = catalog.Default

// liveState is the live search bookkeeping (see statefulBubble.live).
type liveState struct {
	lastKey  time.Time
	interval time.Duration // smoothed time between keystrokes
	inflight []*liveReq
	// mem caches results by scope + normalized query (fresh for a session).
	mem map[string]liveMemEntry
	// exact is set once the list holds network results for the current
	// query, from every source, so a narrowed older result cannot replace
	// them.
	exact map[string]bool // source scope -> answered current query
	// bySource are the current query's network results, per source scope.
	bySource map[string][]*source.Manga
	// picked is set when the user walked the results, so updates keep
	// their highlight instead of jumping back to the top one. It clears
	// once that manga drops out of the results.
	picked bool
	// prefetched remembers chapter lists already requested.
	prefetched map[string]bool
	warming    bool
	// tickSeq is the liveSeq of the trailing request still waiting on its
	// pause, 0 if none.
	tickSeq int
	// lastFire is when the last network request went out.
	lastFire time.Time
	// seeded marks scopes whose reading history is in the catalog.
	seeded map[string]bool
}

type liveMemEntry struct {
	mangas []*source.Manga
	at     time.Time
}

type liveReq struct {
	query  string // normalized
	scope  string
	cancel context.CancelFunc
}

type liveTickMsg struct {
	seq   int
	query string
}

type liveResultMsg struct {
	seq    int
	query  string // as typed
	scope  string
	mangas []*source.Manga
	err    error
	// speculative results only fill the caches.
	speculative bool
}

type liveWarmMsg struct{}

type livePrefetchMsg struct {
	seq int
	url string
}

func (l *liveState) init() {
	if l.mem == nil {
		l.mem = map[string]liveMemEntry{}
		l.exact = map[string]bool{}
		l.bySource = map[string][]*source.Manga{}
		l.prefetched = map[string]bool{}
		l.seeded = map[string]bool{}
	}
}

// liveScope keys caches per source, and per language for MangaDex whose
// titles and results depend on it.
func liveScope(s source.Source) string {
	if s.ID() == mangadex.ID {
		return s.ID() + "|" + viper.GetString(key2.MangadexLanguage)
	}
	return s.ID()
}

// scheduleLive runs on every change of the search text (and on source or
// language changes): it repaints from local data and schedules the
// network.
func (b *statefulBubble) scheduleLive() tea.Cmd {
	l := &b.live
	l.init()
	b.liveSeq++
	seq, q := b.liveSeq, b.inputC.Value()
	nq := catalog.Normalize(q)

	now := time.Now()
	gap := now.Sub(l.lastKey)
	l.lastKey = now
	if gap < liveBurstGap {
		if l.interval == 0 {
			l.interval = gap
		} else {
			l.interval = (3*l.interval + gap) / 4
		}
	}

	l.exact = map[string]bool{}
	l.bySource = map[string][]*source.Manga{}
	b.cancelDiverged(nq)

	if nq == "" {
		b.cancelAll()
		l.tickSeq = 0
		l.picked = false
		b.mangasC.SetItems(nil)
		return nil
	}

	cmds := []tea.Cmd{b.showLocal(q)}
	if !l.warming {
		cmds = append(cmds, b.warmSources())
	}
	if b.allFresh(nq) {
		return tea.Batch(cmds...)
	}
	if gap >= liveBurstGap || now.Sub(l.lastFire) >= liveMaxWait {
		cmds = append(cmds, b.runLive(liveTickMsg{seq: seq, query: q}))
	}
	pause := l.interval * 6 / 5
	if pause < liveMinPause {
		pause = liveMinPause
	} else if pause > liveMaxPause {
		pause = liveMaxPause
	}
	l.tickSeq = seq
	cmds = append(cmds, tea.Tick(pause, func(time.Time) tea.Msg { return liveTickMsg{seq: seq, query: q} }))
	return tea.Batch(cmds...)
}

// allFresh reports whether every source has a fresh cached answer for nq.
func (b *statefulBubble) allFresh(nq string) bool {
	if len(b.selectedSources) == 0 {
		return false
	}
	for _, s := range b.selectedSources {
		if !b.scopeFresh(liveScope(s), nq) {
			return false
		}
	}
	return true
}

// scopeFresh reports whether scope has a fresh answer for nq in memory or
// on disk.
func (b *statefulBubble) scopeFresh(scope, nq string) bool {
	if e, ok := b.live.mem[scope+"\x00"+nq]; ok && time.Since(e.at) < catalog.FreshFor {
		return true
	}
	_, fresh, ok := liveCatalog().Cached(scope, nq)
	return ok && fresh
}

// showLocal repaints the results for q from memory and disk only.
func (b *statefulBubble) showLocal(q string) tea.Cmd {
	nq := catalog.Normalize(q)
	var all []*source.Manga
	cat := liveCatalog()
	for _, s := range b.selectedSources {
		scope := liveScope(s)
		// A cached exact answer (even a stale one, refreshed below) beats
		// narrowing, and late prefix results must not replace it.
		if e, ok := b.live.mem[scope+"\x00"+nq]; ok {
			b.live.exact[scope] = true
			b.live.bySource[scope] = e.mangas
			all = append(all, e.mangas...)
			continue
		}
		if entries, _, ok := cat.Cached(scope, nq); ok {
			mangas := make([]*source.Manga, len(entries))
			for i, e := range entries {
				mangas[i] = e.Manga(s)
			}
			b.live.exact[scope] = true
			b.live.bySource[scope] = mangas
			all = append(all, mangas...)
			continue
		}
		// Narrow what is on screen from this source, then top up from the
		// catalog.
		var mine []*source.Manga
		seen := map[string]bool{}
		for _, it := range b.mangasC.Items() {
			m, ok := it.(*listItem).internal.(*source.Manga)
			if !ok || m.Source == nil || liveScope(m.Source) != scope || seen[m.URL] {
				continue
			}
			if catalog.Matches(q, m.Name, m.Metadata.Synonyms) {
				seen[m.URL] = true
				mine = append(mine, m)
			}
		}
		for _, e := range cat.Search(scope, q, liveLocalLimit) {
			if !seen[e.URL] {
				seen[e.URL] = true
				mine = append(mine, e.Manga(s))
			}
		}
		b.live.bySource[scope] = mine
		all = append(all, mine...)
	}
	return b.setLiveItems(all)
}

// runLive fires the network search for m.query on every selected source.
func (b *statefulBubble) runLive(m liveTickMsg) tea.Cmd {
	if m.seq != b.liveSeq || len(b.selectedSources) == 0 || b.state != searchState {
		return nil
	}
	if b.live.tickSeq == m.seq {
		b.live.tickSeq = 0
	}
	nq := catalog.Normalize(m.query)
	var cmds []tea.Cmd
	for _, s := range b.selectedSources {
		scope := liveScope(s)
		if b.isInflight(scope, nq) || b.scopeFresh(scope, nq) {
			continue
		}
		cmds = append(cmds, b.liveRequest(s, scope, m.seq, m.query, false))
		b.live.lastFire = time.Now()
	}
	if sp := b.speculate(m.query); sp != nil {
		cmds = append(cmds, sp)
	}
	return tea.Batch(cmds...)
}

func (b *statefulBubble) isInflight(scope, nq string) bool {
	for _, r := range b.live.inflight {
		if r.scope == scope && r.query == nq {
			return true
		}
	}
	return false
}

func (b *statefulBubble) liveRequest(s source.Source, scope string, seq int, q string, speculative bool) tea.Cmd {
	l := &b.live
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	req := &liveReq{query: catalog.Normalize(q), scope: scope, cancel: cancel}
	l.inflight = append(l.inflight, req)
	if len(l.inflight) > liveMaxInflight {
		l.inflight[0].cancel()
		l.inflight = l.inflight[1:]
	}
	cat := liveCatalog()
	return func() tea.Msg {
		defer cancel()
		mangas, err := source.SearchContext(ctx, s, q)
		if err == nil {
			cat.Add(scope, q, mangas)
		}
		return liveResultMsg{seq: seq, query: q, scope: scope, mangas: mangas, err: err, speculative: speculative}
	}
}

// speculate prefetches the most likely completion of q from the query
// history, so finishing the word lands on a cached answer.
func (b *statefulBubble) speculate(q string) tea.Cmd {
	if len(b.selectedSources) != 1 {
		return nil
	}
	sug, ok := query.Suggest(q).Get()
	nq, ns := catalog.Normalize(q), catalog.Normalize(sug)
	if !ok || ns == nq || !strings.HasPrefix(ns, nq) {
		return nil
	}
	s := b.selectedSources[0]
	scope := liveScope(s)
	if _, cached := b.live.mem[scope+"\x00"+ns]; cached || b.isInflight(scope, ns) {
		return nil
	}
	if _, fresh, ok := liveCatalog().Cached(scope, ns); ok && fresh {
		return nil
	}
	return b.liveRequest(s, scope, b.liveSeq, sug, true)
}

// cancelDiverged cancels requests unrelated to the current text. Those for
// a prefix keep running (their results narrow down), as do those for an
// extension (a speculative completion the user may still type, or the
// query before a backspace, whose answer gets cached).
func (b *statefulBubble) cancelDiverged(nq string) {
	l := &b.live
	keep := l.inflight[:0]
	for _, r := range l.inflight {
		if nq != "" && (strings.HasPrefix(nq, r.query) || strings.HasPrefix(r.query, nq)) {
			keep = append(keep, r)
		} else {
			r.cancel()
		}
	}
	l.inflight = keep
}

func (b *statefulBubble) cancelAll() {
	for _, r := range b.live.inflight {
		r.cancel()
	}
	b.live.inflight = nil
}

func (b *statefulBubble) dropInflight(scope, nq string) {
	l := &b.live
	for i, r := range l.inflight {
		if r.scope == scope && r.query == nq {
			l.inflight = append(l.inflight[:i], l.inflight[i+1:]...)
			return
		}
	}
}

func (b *statefulBubble) applyLive(m liveResultMsg) tea.Cmd {
	l := &b.live
	l.init()
	nq := catalog.Normalize(m.query)
	b.dropInflight(m.scope, nq)
	if m.err != nil {
		if !errors.Is(m.err, context.Canceled) && !errors.Is(m.err, context.DeadlineExceeded) {
			log.Error(m.err)
		}
		return nil
	}
	l.mem[m.scope+"\x00"+nq] = liveMemEntry{mangas: m.mangas, at: time.Now()}
	if b.state != searchState {
		return nil
	}
	cur := b.inputC.Value()
	ncur := catalog.Normalize(cur)
	if m.speculative && nq != ncur {
		// Only warms the caches, unless typing caught up with it. Fetch the
		// top covers to disk too, so they paint at once if it is typed.
		go prefetchCovers(m.mangas, liveCoverPrefetch)
		return nil
	}
	switch {
	case nq == ncur:
		l.exact[m.scope] = true
		l.bySource[m.scope] = m.mangas
	case ncur != "" && strings.HasPrefix(ncur, nq) && !l.exact[m.scope]:
		// An older, shorter query answered first: narrow it down.
		var narrowed []*source.Manga
		for _, mg := range m.mangas {
			if catalog.Matches(cur, mg.Name, mg.Metadata.Synonyms) {
				narrowed = append(narrowed, mg)
			}
		}
		if len(narrowed) == 0 {
			return nil
		}
		// Keep catalog-only matches after the narrowed network ones.
		seen := map[string]bool{}
		for _, mg := range narrowed {
			seen[mg.URL] = true
		}
		for _, mg := range l.bySource[m.scope] {
			if !seen[mg.URL] {
				narrowed = append(narrowed, mg)
			}
		}
		l.bySource[m.scope] = narrowed
	default:
		return nil
	}
	var all []*source.Manga
	for _, s := range b.selectedSources {
		all = append(all, l.bySource[liveScope(s)]...)
	}
	return tea.Batch(b.setLiveItems(all), b.schedulePrefetch())
}

// setLiveItems replaces the result list, reusing the rows (and so the
// covers) of mangas still present and keeping the highlight on the same
// manga when the user picked one. Identical lists are left alone.
func (b *statefulBubble) setLiveItems(mangas []*source.Manga) tea.Cmd {
	old := b.mangasC.Items()
	var selURL string
	if it, ok := b.mangasC.SelectedItem().(*listItem); ok {
		if m, ok := it.internal.(*source.Manga); ok {
			selURL = m.URL
		}
	}
	byURL := make(map[string]*listItem, len(old))
	for _, it := range old {
		li := it.(*listItem)
		if m, ok := li.internal.(*source.Manga); ok {
			byURL[m.URL] = li
		}
	}

	items := make([]list.Item, 0, len(mangas))
	seen := make(map[string]bool, len(mangas))
	same := len(old) > 0
	for _, m := range mangas {
		if seen[m.URL] {
			continue
		}
		seen[m.URL] = true
		li := byURL[m.URL]
		if li == nil {
			li = &listItem{internal: m}
		} else if prev := li.internal.(*source.Manga); prev != m {
			// Prefer the network copy over a catalog one (it carries the
			// source's own state), keeping the row itself.
			li.internal = m
		}
		if i := len(items); same && (i >= len(old) || old[i] != li) {
			same = false
		}
		items = append(items, li)
	}
	if same && len(items) == len(old) {
		return nil
	}
	cmd := b.mangasC.SetItems(items)
	idx := 0
	if b.live.picked {
		b.live.picked = false
		for i, it := range items {
			if it.(*listItem).internal.(*source.Manga).URL == selURL {
				idx = i
				b.live.picked = true
				break
			}
		}
	}
	b.mangasC.Select(idx)
	return cmd
}

// schedulePrefetch fetches the highlighted result's chapter list once it
// has stayed highlighted for a moment, so enter opens it instantly.
func (b *statefulBubble) schedulePrefetch() tea.Cmd {
	it, ok := b.mangasC.SelectedItem().(*listItem)
	if !ok {
		return nil
	}
	m, ok := it.internal.(*source.Manga)
	if !ok || m.Source == nil || b.live.prefetched[m.URL] || !source.ConcurrencySafe(m.Source) {
		return nil
	}
	seq, u := b.liveSeq, m.URL
	return tea.Tick(livePrefetchDelay, func(time.Time) tea.Msg { return livePrefetchMsg{seq: seq, url: u} })
}

func (b *statefulBubble) runPrefetch(m livePrefetchMsg) tea.Cmd {
	if m.seq != b.liveSeq || b.state != searchState {
		return nil
	}
	it, ok := b.mangasC.SelectedItem().(*listItem)
	if !ok {
		return nil
	}
	mg, ok := it.internal.(*source.Manga)
	if !ok || mg.URL != m.url || b.live.prefetched[mg.URL] {
		return nil
	}
	b.live.prefetched[mg.URL] = true
	return func() tea.Msg {
		_, _ = mg.Source.ChaptersOf(mg)
		return nil
	}
}

// warmSources opens connections to the selected sources now, and again
// every liveWarmEvery while the search screen is up.
func (b *statefulBubble) warmSources() tea.Cmd {
	b.live.init()
	srcs := b.selectedSources
	scopes := make(map[string]string, len(srcs)) // source ID -> scope
	for _, s := range srcs {
		if scope := liveScope(s); !b.live.seeded[scope] {
			b.live.seeded[scope] = true
			scopes[s.ID()] = scope
		}
	}
	// Load the catalog off the first keystroke's path, and make sure every
	// manga in the reading history is findable locally.
	getCat := liveCatalog // read the hook here, load lazily off-thread
	go func() {
		cat := getCat()
		if len(scopes) == 0 {
			return
		}
		saved, err := history.Get()
		if err != nil {
			return
		}
		for _, c := range saved {
			if scope, ok := scopes[c.SourceID]; ok {
				cat.Seed(scope, c.MangaName, c.MangaURL, c.MangaID)
			}
		}
	}()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, s := range srcs {
			if w, ok := s.(source.Warmer); ok {
				go func() { _ = w.Warm(ctx) }()
			}
		}
		<-ctx.Done()
	}()
	if b.live.warming {
		return nil
	}
	b.live.warming = true
	return tea.Tick(liveWarmEvery, func(time.Time) tea.Msg { return liveWarmMsg{} })
}

func (b *statefulBubble) handleWarm() tea.Cmd {
	b.live.warming = false
	if b.state != searchState {
		// Re-armed by the next scheduleLive-driven warm.
		return nil
	}
	return b.warmSources()
}

// livePending reports whether a network answer for the current text is
// still on its way (shown as a faint marker in the search bar).
func (b *statefulBubble) livePending() bool {
	nq := catalog.Normalize(b.inputC.Value())
	if nq == "" || b.state != searchState {
		return false
	}
	if b.live.tickSeq != b.liveSeq && len(b.live.inflight) == 0 {
		return false // nothing scheduled or running (a failed request)
	}
	for _, s := range b.selectedSources {
		if !b.scopeFresh(liveScope(s), nq) {
			return true
		}
	}
	return false
}

// viewLive is the search screen: a full-width search bar on top and the
// live manga results below it. Before anything is
// typed the home screen (art + keys) shows instead.
func (b *statefulBubble) viewLive() string {
	full := b.width + 4
	boxW := imax(full-4, 12)
	b.inputC.Width = imax(boxW-6-lipgloss.Width(b.inputC.Prompt), 4)
	box := b.searchBox(boxW)
	for lipgloss.Width(box) > full-2 && boxW > 12 {
		boxW--
		b.inputC.Width = imax(boxW-6-lipgloss.Width(b.inputC.Prompt), 4)
		box = b.searchBox(boxW)
	}
	box = lipgloss.PlaceHorizontal(full, lipgloss.Center, box)

	header := box
	pickerW := lipgloss.Width(b.searchBox(boxW))
	var drops []string
	if b.dropdown != nil {
		drops = append(drops, b.viewDropdown(pickerW))
	}
	if b.showKeys {
		// Keybinds always stack below an open source/language picker.
		drops = append(drops, b.viewKeysBox(pickerW))
	}
	if len(drops) > 0 {
		// Expansions float over whatever is under the search bar
		// (art, results) so nothing below them moves.
		drop := lipgloss.PlaceHorizontal(full, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Left, drops...))
		return overlayLines(b.viewLiveBase(box, full, pickerW), drop, lipgloss.Height(box))
	}
	return b.viewLiveBase(header, full, pickerW)
}

// overlayLines replaces lines of base starting at row with the lines of top.
func overlayLines(base, top string, row int) string {
	bl := strings.Split(base, "\n")
	for i, l := range strings.Split(top, "\n") {
		if row+i < len(bl) {
			bl[row+i] = l
		} else {
			bl = append(bl, l)
		}
	}
	return strings.Join(bl, "\n")
}

// viewLiveBase renders the search screen under header, ignoring dropdowns.
func (b *statefulBubble) viewLiveBase(header string, full, pickerW int) string {
	// Hero layout (art + boxes) until there are results to show.
	homeView := b.state == loadingState ||
		(b.state == searchState && (b.inputC.Value() == "" || len(b.mangasC.Items()) == 0))
	footer := ""
	if len(b.sourceChoices) > 0 && homeView {
		footer = lipgloss.PlaceHorizontal(full, lipgloss.Center, b.sourcePicker(pickerW))
	}

	H := b.height + 2 - lipgloss.Height(header)
	if b.state == loadingState {
		// Loader bar sits right under the search box.
		bar := lipgloss.PlaceHorizontal(full, lipgloss.Center, b.loaderC.view(pickerW))
		header += "\n" + bar
		H -= lipgloss.Height(bar)
	}
	if homeView {
		// Empty bar: art and keys fill the middle, the status box sits at
		// the very bottom.
		if footer == "" {
			return header + "\n" + b.homeBody(full, H)
		}
		// Downloaded box stacks above the status box, unless it would
		// push the art off a screen where the art otherwise fits.
		if dl := lipgloss.PlaceHorizontal(full, lipgloss.Center, b.downloadedBox(pickerW)); dl != "" {
			artH := len(homeArt)
			withDl := H - lipgloss.Height(footer) - lipgloss.Height(dl) - 1
			withoutDl := H - lipgloss.Height(footer) - 1
			if withDl >= artH || withoutDl < artH {
				footer = dl + "\n" + footer
			}
		}
		bodyH := H - lipgloss.Height(footer) - 1
		return header + "\n" + b.homeBody(full, imax(bodyH, 0)) + "\n" + footer
	}
	if H < 4 {
		return header
	}
	if len(b.mangasC.Items()) == 0 {
		// No results (or still fetching): keep the art on screen.
		return header + "\n" + b.homeBody(full, H-1)
	}
	// Card height comes from the resting layout (collapsed bar, five
	// rows), so an open dropdown drops rows rather than shrinking them.
	restGridH := b.height + 2 - 3 - 2
	b.grid.fixedCardH = imax(restGridH/singleColRows, 2)
	defer func() { b.grid.fixedCardH = 0 }()
	grid := b.viewGridH(&b.mangasC, mangaCard, func(th *thumbs, it list.Item, prio int) {
		th.WantManga(it.(*listItem).internal.(*source.Manga), prio)
	}, H, true)
	return header + "\n" + grid
}
