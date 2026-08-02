// picker is a compact scrolling chooser with optional type-to-filter, shared
// by every "pick one of these" step in svrctl's interactive screens so they
// all look and behave the same.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/adammcgrogan/svrctl/internal/ui"
)

// Item is one selectable row. Desc is optional supporting text.
type Item struct {
	Title string
	Desc  string
	// Badge is a short marker rendered after the title, e.g. "latest".
	Badge string
}

type picker struct {
	items    []Item
	filtered []int // indices into items matching the current filter
	cursor   int   // index into filtered
	offset   int   // first visible row of filtered
	height   int   // visible rows
	filter   string
	// filterable enables type-to-filter. Short menus do not need it; a list of
	// three hundred Minecraft versions very much does.
	filterable bool
}

func newPicker(items []Item, height int, filterable bool) picker {
	p := picker{items: items, height: height, filterable: filterable}
	p.applyFilter()
	return p
}

func (p *picker) setItems(items []Item) {
	p.items = items
	p.cursor, p.offset, p.filter = 0, 0, ""
	p.applyFilter()
}

func (p *picker) applyFilter() {
	p.filtered = p.filtered[:0]
	needle := strings.ToLower(p.filter)
	for i, it := range p.items {
		if needle == "" || strings.Contains(strings.ToLower(it.Title), needle) {
			p.filtered = append(p.filtered, i)
		}
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = max(len(p.filtered)-1, 0)
	}
	p.clampOffset()
}

func (p *picker) clampOffset() {
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+p.height {
		p.offset = p.cursor - p.height + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

// selectTitle moves the cursor to the item with the given title, if present,
// so a value supplied by flag shows up as the current choice.
func (p *picker) selectTitle(title string) {
	for i, idx := range p.filtered {
		if p.items[idx].Title == title {
			p.cursor = i
			p.clampOffset()
			return
		}
	}
}

// selected returns the highlighted item, or false when the filter matches none.
func (p picker) selected() (Item, bool) {
	if p.cursor < 0 || p.cursor >= len(p.filtered) {
		return Item{}, false
	}
	return p.items[p.filtered[p.cursor]], true
}

// update handles navigation and filter typing, returning whether it consumed
// the key. Callers check that before interpreting the key themselves.
func (p *picker) update(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlP:
		if p.cursor > 0 {
			p.cursor--
			p.clampOffset()
		}
		return true
	case tea.KeyDown, tea.KeyCtrlN:
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
			p.clampOffset()
		}
		return true
	case tea.KeyPgUp:
		p.cursor = max(p.cursor-p.height, 0)
		p.clampOffset()
		return true
	case tea.KeyPgDown:
		p.cursor = min(p.cursor+p.height, max(len(p.filtered)-1, 0))
		p.clampOffset()
		return true
	case tea.KeyHome:
		p.cursor, p.offset = 0, 0
		return true
	case tea.KeyEnd:
		p.cursor = max(len(p.filtered)-1, 0)
		p.clampOffset()
		return true
	}

	if !p.filterable {
		return false
	}
	switch msg.Type {
	case tea.KeyBackspace:
		if p.filter != "" {
			p.filter = p.filter[:len(p.filter)-1]
			p.applyFilter()
		}
		return true
	case tea.KeyRunes, tea.KeySpace:
		p.filter += string(msg.Runes)
		p.applyFilter()
		return true
	}
	return false
}

func (p picker) view(width int) string {
	if len(p.filtered) == 0 {
		return "  " + ui.Subtle.Render("nothing matches “"+p.filter+"”")
	}

	var b strings.Builder
	end := min(p.offset+p.height, len(p.filtered))
	for row := p.offset; row < end; row++ {
		it := p.items[p.filtered[row]]

		selected := row == p.cursor
		if selected {
			b.WriteString(ui.Selected.Render(ui.GlyphPrompt + " " + it.Title))
		} else {
			b.WriteString("  " + ui.Body.Render(it.Title))
		}
		if it.Badge != "" {
			badge := ui.Subtle
			if selected {
				badge = ui.Success
			}
			b.WriteString("  " + badge.Render(it.Badge))
		}
		if it.Desc != "" {
			b.WriteString("  " + ui.Subtle.Render(it.Desc))
		}
		b.WriteString("\n")
	}

	// Tell the user there is more below rather than letting the list look
	// complete when it has been cut off.
	if len(p.filtered) > p.height {
		b.WriteString("  " + ui.Faint.Render(fmt.Sprintf("%d/%d", p.cursor+1, len(p.filtered))))
		b.WriteString("\n")
	}
	return b.String()
}
