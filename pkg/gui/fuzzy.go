package gui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sahilm/fuzzy"
)

func FuzzyFind(inputField *tview.InputField, focusedList *tview.List, listItems []string, b *tview.List, f *tview.List, p *tview.TextView) {
	inputField.SetChangedFunc(func(text string) {
		results := fuzzy.Find(text, listItems)
		focusedList.Clear()
		for _, val := range results {
			focusedList.AddItem(val.Str, "", 0, nil)
		}
	})
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			text := inputField.GetText()
			if len(text) < 2 {
				focusedList.Clear()
				for _, val := range listItems {
					focusedList.AddItem(val, "", 0, nil)
				}
			} else {
				results := fuzzy.Find(text, listItems)
				focusedList.Clear()
				for _, val := range results {
					focusedList.AddItem(val.Str, "", 0, nil)
				}
			}
		} else if event.Key() == tcell.KeyDown {
			ScrollDown(focusedList)
		} else if event.Key() == tcell.KeyUp {
			ScrollUp(focusedList)
		} else if event.Key() == tcell.KeyEnter {
			if focusedList.GetItemCount() == 0 {
				for _, val := range listItems {
					focusedList.AddItem(val, "", 0, nil)
				}
				footer := createDefaultFooter(envName)
				grid := CreateDefaultGrid(b, f, p, footer)
				app.SetRoot(grid, true).SetFocus(focusedList)
				return event
			}
			text, _ := focusedList.GetItemText(focusedList.GetCurrentItem())
			focusedList.Clear()
			for _, val := range listItems {
				focusedList.AddItem(val, "", 0, nil)
			}
			targetIndex = -1
			for i := 0; i < focusedList.GetItemCount(); i++ {
				mainText, _ := focusedList.GetItemText(i)
				if mainText == text {
					targetIndex = i
					focusedList.SetCurrentItem(targetIndex)
					break
				}
			}

			footer := createDefaultFooter(envName)
			grid := CreateDefaultGrid(b, f, p, footer)

			app.SetRoot(grid, true).SetFocus(focusedList)

			if targetIndex != -1 {
				focusedList.SetCurrentItem(targetIndex)
				app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			}
		} else if event.Key() == tcell.KeyEscape {

			focusedList.Clear()
			for _, val := range listItems {
				focusedList.AddItem(val, "", 0, nil)
			}

			footer := createDefaultFooter(envName)
			grid := CreateDefaultGrid(b, f, p, footer)

			app.SetRoot(grid, true).SetFocus(focusedList)
		}
		return event
	})
}

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
