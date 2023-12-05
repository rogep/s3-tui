package gui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/rogep/s3-tui/pkg/awslib"
	"github.com/rogep/s3-tui/pkg/utils"
)

var (
	app            *tview.Application
	bucketName     string
	envName        string
	currentFocus   string // allows refocusing when exiting forms/new app state
	selectedFile   string
	initialBuckets []string // used for fuzzy finding as we clear the bucket list and lose state
	initialFiles   []string // used for fuzzy finding as we clear the files list and lose state
	targetIndex    int
)

func CreateGridWithSearch(buckets *tview.List, files *tview.List, preview *tview.TextView, footer *tview.InputField) *tview.Grid {
	grid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0, -2, 0).
		SetBorders(false).
		AddItem(tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetDynamicColors(true).
			SetText(""), 0, 0, 1, 3, 0, 0, false).
		AddItem(footer, 2, 0, 1, 3, 0, 0, false)

	grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
		AddItem(files, 1, 1, 1, 1, 0, 100, false).
		AddItem(preview, 1, 2, 1, 1, 0, 100, false)

	return grid
}

func CreateDefaultGrid(buckets *tview.List, files *tview.List, preview *tview.TextView, footer *tview.TextView) *tview.Grid {
	grid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0, 0, 0).
		SetBorders(false).
		AddItem(tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetDynamicColors(true).
			SetText(""), 0, 0, 1, 3, 0, 0, false).
		AddItem(footer, 2, 0, 1, 3, 0, 0, false)

	grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
		AddItem(files, 1, 1, 1, 1, 0, 100, false).
		AddItem(preview, 1, 2, 1, 1, 0, 100, false)

	return grid
}

func createDefaultFooter(envName string) *tview.TextView {
	parts := []string{
		"Credentials: [yellow]%s[white] - Shortcuts: ([green]/[white])search |",
		" ([green]ESC[white])ape | <[green]Ctrl+[white]> ([green]c[white])reate bucket |",
		" ([green]a[white])dd Credentials | ([green]d[white])elete | ([green]r[white])ename |",
		" ([green]u[white])pload | ([green]s[white])wap credentials",
	}

	footerText := strings.Join(parts, "")
	footerText = fmt.Sprintf(footerText, envName)

	return tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetText(footerText)
}

func spinTitle(app *tview.Application, box *tview.List, title string, action func()) {
	done := make(chan bool)

	// Remember the original title
	originalTitle := box.GetTitle()

	// action
	go func() {
		action()
		done <- true
		close(done)
	}()

	// spinner
	go func() {
		spinners := []string{"/ ", "| ", "\\ ", "- ", "/ "}
		dots := []string{"     ", ".    ", "..   ", "...  ", ".... ", "....."}
		var i int
		j := -1 // lazy hack to allow us to start at index 0
		for {
			select {
			case _ = <-done:
				app.QueueUpdateDraw(func() {
					box.SetTitle(originalTitle) // Restore original title
				})
				return
			case <-time.After(150 * time.Millisecond):
				spin := i % len(spinners)
				if i%len(spinners) == 0 {
					j++
					j = j % len(dots)
				}
				app.QueueUpdateDraw(func() {
					box.SetTitle(title + dots[j] + spinners[spin])
				})
				i++
			}
		}
	}()
}

func S3Gui(s *awslib.S3Handler, envName string) {
	res, err := s.GetBuckets()
	if err != nil {
		panic(err)
	}
	app = tview.NewApplication()
	buckets := tview.NewList().ShowSecondaryText(false)
	for _, val := range res {
		buckets.AddItem(val, "", 0, nil)
		initialBuckets = append(initialBuckets, val)
	}

	// SetBackgroundColor(tcell.ColorDefault)
	buckets.SetBorder(true).SetTitle("Buckets <Ctrl+b>").SetBorderColor(tcell.ColorYellow)
	preview := tview.NewTextView().SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	preview.SetBorder(true).SetTitle("Preview <Ctrl+p>").SetBorderColor(tcell.ColorWhite)
	files := tview.NewList()
	files.ShowSecondaryText(false).
		SetDoneFunc(func() {
			files.Clear()
			preview.Clear()
			app.SetFocus(buckets)
		})
	files.SetBorder(true).SetTitle("Files <Ctrl+f>").SetBorderColor(tcell.ColorWhite)
	currentFocus = "buckets"

	// LIST ACTIONS
	buckets.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		currentFocus = "files"
		selectedBucket := mainText
		bucketName = selectedBucket

		result, err := s.GetDirectoryStructure(bucketName, "/", "")
		initialFiles = result
		if err != nil {
			panic(err)
		}

		files.Clear()
		preview.Clear()
		app.SetFocus(files)
		files.SetBorderColor(tcell.ColorYellow)
		buckets.SetBorderColor(tcell.ColorWhite)

		for _, val := range result {
			if val == "" {
				continue
			}
			files.AddItem(val, "", 0, nil)
		}
	})

	files.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if files.GetItemCount() == 0 {
			return event
		}
		currentFocus = "files"
		selectedItemIndex := files.GetCurrentItem()
		selectedKey, _ := files.GetItemText(selectedItemIndex)
		switch event.Key() {
		case tcell.KeyCtrlD:
			res, err := s.DeleteObject(bucketName, selectedKey)
			if res == false {
				return event
			}
			var prefix string
			splitKey := strings.Split(selectedKey, "/")
			if len(splitKey) == 1 {
				prefix = ""
			} else {
				prefix = strings.Join(splitKey[:len(splitKey)-1], "/") + "/"
			}

			result, err := s.GetDirectoryStructure(bucketName, "/", prefix)
			initialFiles = result
			if err != nil {
				panic(err)
			}
			files.Clear()
			preview.Clear()
			app.SetFocus(files)
			files.SetBorderColor(tcell.ColorYellow)
			buckets.SetBorderColor(tcell.ColorWhite)
			for _, val := range result {
				files.AddItem(val, "", 0, nil)
			}

			// TODO: remove key in rename
		case tcell.KeyCtrlR:
			if selectedKey == ".." || selectedKey[len(selectedKey)-1:] == "/" {
				footer := createDefaultFooter(envName)
				grid := CreateDefaultGrid(buckets, files, preview, footer)
				app.SetRoot(grid, true).SetFocus(files)
				return event
			}
			renameInput := tview.NewInputField().
				SetLabel("Rename: ").
				SetFieldWidth(100)
			grid := CreateGridWithSearch(buckets, files, preview, renameInput)
			app.SetRoot(grid, true).SetFocus(renameInput)

			renameInput.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					_, err := s.RenameObject(bucketName, selectedKey, renameInput.GetText())
					if err != nil {
						panic(err)
					}
					var prefix string
					splitKey := strings.Split(selectedKey, "/")
					if len(splitKey) == 1 {
						prefix = ""
					} else {
						prefix = strings.Join(splitKey[:len(splitKey)-1], "/") + "/"
					}
					result, err := s.GetDirectoryStructure(bucketName, "/", prefix)
					initialFiles = result
					if err != nil {
						panic(err)
					}

					files.Clear()
					preview.Clear()
					app.SetFocus(files)
					files.SetBorderColor(tcell.ColorYellow)
					buckets.SetBorderColor(tcell.ColorWhite)
					for _, val := range result {
						files.AddItem(val, "", 0, nil)
					}

					footer := createDefaultFooter(envName)
					grid := CreateDefaultGrid(buckets, files, preview, footer)
					app.SetRoot(grid, true).SetFocus(files)
				}
			})
		}

		return event
	})
	files.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		currentFocus = "files"
		selectedKey := mainText
		if selectedKey != ".." {
			selectedFile = selectedKey
		} else {
			splitKey := strings.Split(selectedFile, "/")
			if len(splitKey) > 2 {
				splitKey = splitKey[:len(splitKey)-2]
				selectedKey = strings.Join(splitKey, "/") + "/"
			} else {
				selectedKey = ""
			}
			selectedFile = selectedKey
		}

		if selectedKey == "" {
			files.Clear()
			res, err := s.GetDirectoryStructure(bucketName, "/", selectedKey)
			initialFiles = res
			if err != nil {
				panic(err)
			}
			for _, val := range res {
				files.AddItem(val, "", 0, nil)
			}
		} else if selectedKey[len(selectedKey)-1:] == "/" {
			files.Clear()
			res, err := s.GetDirectoryStructure(bucketName, "/", selectedKey)
			initialFiles = res
			if err != nil {
				panic(err)
			}
			for _, val := range res {
				files.AddItem(val, "", 0, nil)
			}

		} else {
			glacier, err := s.IsGlacier(bucketName, selectedKey)
			if err != nil {
				panic(err)
			}
			if glacier {
				preview.SetText(string("Cannot view a file stored in Glacier. Please restore the file if you wish to view."))
			} else {
				byteContent, err := s.PreviewFile(bucketName, selectedKey)
				if err != nil {
					// TODO: fix error handling
					panic(err)
				}
				byteContent = utils.ParsePreview(byteContent)
				preview.SetText(string(byteContent))
			}
		}
	})

	footer := createDefaultFooter(envName)
	grid := CreateDefaultGrid(buckets, files, preview, footer)

	// TODO: figure out a nice way to do reactivity
	// // Layout for screens narrower than 100 cells (menu and side bar are hidden).
	// grid.AddItem(buckets, 0, 0, 0, 0, 0, 0, false).
	// 	AddItem(files, 0, 0, 0, 0, 0, 0, false).
	// 	AddItem(preview, 0, 0, 0, 0, 0, 0, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlB:
			app.SetFocus(buckets)
			buckets.SetBorderColor(tcell.ColorYellow)
			files.SetBorderColor(tcell.ColorWhite)
			preview.SetBorderColor(tcell.ColorWhite)
			currentFocus = "buckets"
		case tcell.KeyCtrlF:
			app.SetFocus(files)
			files.SetBorderColor(tcell.ColorYellow)
			buckets.SetBorderColor(tcell.ColorWhite)
			preview.SetBorderColor(tcell.ColorWhite)
			currentFocus = "files"
		case tcell.KeyCtrlP:
			app.SetFocus(preview)
			preview.SetBorderColor(tcell.ColorYellow)
			buckets.SetBorderColor(tcell.ColorWhite)
			files.SetBorderColor(tcell.ColorWhite)
		case tcell.KeyCtrlE:
			form := tview.NewForm().
				AddInputField("Name", "", 40, nil, nil).
				AddInputField("Access Key", "", 40, nil, nil).
				AddPasswordField("Secret Access Key", "", 40, '*', nil).
				AddInputField("SSO (Optional)", "", 40, nil, nil).
				AddTextView("Note", "Credentials will be stored inside\n.aws/credentials", 40, 2, true, false).
				AddButton("Save", func() {
					// add env logic here
					app.SetRoot(grid, true).EnableMouse(true).Run()
				}).
				AddButton("Quit", func() {
					app.SetRoot(grid, true).EnableMouse(true).Run()
				})
			form.SetBorder(true).SetTitle("AWS Credentials Configuration").SetTitleAlign(tview.AlignLeft)
			app.SetRoot(form, true).EnableMouse(true).Run()

		case tcell.KeyCtrlT:
			bucketInput := tview.NewInputField().
				SetLabel(fmt.Sprintf("Enter bucket name: ")).
				SetFieldWidth(54)

			grid := CreateGridWithSearch(buckets, files, preview, bucketInput)
			app.SetRoot(grid, true).SetFocus(bucketInput)

			bucketInput.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					spinTitle(app, buckets, "Creating bucket", func() {
						_, err := s.CreateBucket(bucketInput.GetText(), 8)
						if err != nil {
							panic("lame")
						}
					})
					res, err := s.GetBuckets()
					if err != nil {
						panic(err)
					}

					buckets.Clear()
					buckets.SetTitle("Buckets <Ctrl+b")
					for _, val := range res {
						buckets.AddItem(val, "", 0, nil)
					}

					footer := createDefaultFooter(envName)
					grid := CreateDefaultGrid(buckets, files, preview, footer)
					app.SetRoot(grid, true).SetFocus(buckets)
				}
			})

			// nested switch is needed to use '/' (or skill issue). LETS GOOOOOOOOOOO
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				renameInput := tview.NewInputField().
					SetLabel("Search: ").
					SetFieldWidth(100)

				grid := CreateGridWithSearch(buckets, files, preview, renameInput)

				app.SetRoot(grid, true).SetFocus(renameInput)
				if currentFocus == "buckets" {
					FuzzyFind(renameInput, buckets, initialBuckets, buckets, files, preview)
				} else if currentFocus == "files" {
					FuzzyFind(renameInput, files, initialFiles, buckets, files, preview)
				} else {
					fmt.Println("Focus is not on a tview.List")
				}
			}

		case tcell.KeyCtrlQ:
			modal := tview.NewModal().
				SetText("Do you want to quit s3-tui?").
				AddButtons([]string{"Quit", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "Quit" {
						app.Stop()
						os.Exit(0)
					}
					if buttonLabel == "Cancel" {
						app.SetRoot(grid, true).EnableMouse(true).Run()
					}
				})
			app.SetRoot(modal, true).EnableMouse(true).Run()

		case tcell.KeyCtrlU:
			var mu sync.Mutex
			mu.Lock()
			defer mu.Unlock()
			spinTitle(app, files, "Uploading", func() {
				// swap this with s3 upload
				time.Sleep(10 * time.Second)
			})

		}
		return event
	})

	// TODO: figure out how to change colours based on click events
	if err := app.SetRoot(grid, true).EnableMouse(false).SetFocus(buckets).Run(); err != nil {
		panic(err)
	}
}
