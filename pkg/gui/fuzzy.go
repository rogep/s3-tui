package gui

import (
	"github.com/rivo/tview"
	_ "github.com/sahilm/fuzzy"
)

func ScrollDown(l *tview.List) {
	count := l.GetItemCount()
	index := l.GetCurrentItem()
	index += 1
	l.SetCurrentItem(index % count)
}

func ScrollUp(l *tview.List) {
	count := l.GetItemCount()
	index := l.GetCurrentItem()
	index -= 1
	l.SetCurrentItem(index % count)
}
