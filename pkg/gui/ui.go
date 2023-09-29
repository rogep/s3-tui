package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app        *tview.Application // The tview application.
	bucketName string
	envName    string
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()

	// Add descriptions for positional arguments
	fmt.Println("\nPositional Arguments:")
	fmt.Println("  arg1        AWS Access Key ID")
	fmt.Println("  arg2        AWS Secret Access Key")
	fmt.Println("  arg3        AWS SSO (Optional)")
}

func main() {
	// TODO: Consider using some CLI for creds before we do a pop up
	// with no flags set, the application will look at the default profile
	envPtr := flag.Bool("E", false, "Use AWS credentials from environment variables")
	flag.Usage = usage
	flag.Parse()
	fmt.Println(os.Environ())

	var awsConfig aws.Config

	if (len(flag.Args()) > 3 || len(flag.Args()) == 1) && *envPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) > 1 && !*envPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) == 2 && *envPtr {
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(flag.Args()[0], flag.Args()[1], ""),
		}
		envName = "cli"
	} else if len(flag.Args()) == 3 && *envPtr {
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(flag.Args()[0], flag.Args()[1], flag.Args()[2]),
		}
		envName = "cli"
	} else {
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials("", "", ""),
		}
		envName = "Default"
		// TODO: add env var parser and .config/credentials default profile selection
	}

	sess, err := session.NewSession(&awsConfig)
	if err != nil {
		panic(err)
	}
	svc := s3.New(sess)
	input := &s3.ListBucketsInput{}

	res, err := svc.ListBuckets(input)
	if err != nil {
		panic(err)
	}

	app = tview.NewApplication()

	buckets := tview.NewList().ShowSecondaryText(false)
	for _, val := range res.Buckets {
		buckets.AddItem(*val.Name, "", 0, nil)
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
	buckets.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selectedBucket := mainText
		bucketName = selectedBucket
		input2 := &s3.ListObjectsV2Input{
			Bucket:  aws.String(selectedBucket),
			MaxKeys: aws.Int64(1000),
		}

		result, err := svc.ListObjectsV2(input2)
		if err != nil {
			panic(err)
		}
		files.Clear()
		preview.Clear()
		app.SetFocus(files)
		files.SetBorderColor(tcell.ColorYellow)
		buckets.SetBorderColor(tcell.ColorWhite)
		for _, val := range result.Contents {
			if *val.Size == 0 {
				continue
			}
			files.AddItem(*val.Key, "", 0, nil)
		}
	})
	files.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selectedBucket := mainText
		output, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(selectedBucket),
			Range:  aws.String("bytes=0-1000"),
		})
		if err != nil {
			fmt.Println("Error getting object ", err)
		}

		// Convert the content to byte slice
		buf := new(bytes.Buffer)
		buf.ReadFrom(output.Body)
		byteContent := buf.Bytes()
		preview.SetText(string(byteContent))
	})

	// TODO: figure out how to create a helper footer
	// ALSO NEED an environment/creds selector!!

	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignLeft).
			SetDynamicColors(true).
			SetText(text)
	}
	// TODO: use this below to add the environment profile name that you are on
	grid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(50, 50, 0).
		SetBorders(false).
		AddItem(newPrimitive(""), 0, 0, 1, 3, 0, 0, false).
		AddItem(newPrimitive(fmt.Sprintf("Environment: [yellow]%s[white] - Shortcuts: ([green]/[white])search | <[green]Ctrl+[white]> ([green]q[white])uit | ([green]h[white])elp | ([green]e[white])nvironments | ([green]a[white])dd Environment | ([green]d[white])elete | ([green]r[white])ename | ([green]u[white])pload", envName)), 2, 0, 1, 3, 0, 0, false)

	// // Layout for screens narrower than 100 cells (menu and side bar are hidden).
	// grid.AddItem(buckets, 0, 0, 0, 0, 0, 0, false).
	// 	AddItem(files, 0, 0, 0, 0, 0, 0, false).
	// 	AddItem(preview, 0, 0, 0, 0, 0, 0, false)

	// Layout for screens wider than 100 cells.
	grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
		AddItem(files, 1, 1, 1, 1, 0, 100, false).
		AddItem(preview, 1, 2, 1, 1, 0, 100, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlB:
			app.SetFocus(buckets)
			buckets.SetBorderColor(tcell.ColorYellow)
			files.SetBorderColor(tcell.ColorWhite)
			preview.SetBorderColor(tcell.ColorWhite)
		case tcell.KeyCtrlF:
			app.SetFocus(files)
			files.SetBorderColor(tcell.ColorYellow)
			buckets.SetBorderColor(tcell.ColorWhite)
			preview.SetBorderColor(tcell.ColorWhite)
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

		case tcell.KeyEscape | tcell.KeyCtrlQ:
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

		}
		return event
	})

	// TODO: figure out how to change colours based on click events
	if err := app.SetRoot(grid, true).EnableMouse(false).SetFocus(buckets).Run(); err != nil {
		panic(err)
	}
}
