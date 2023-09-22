package main

import (
	"bytes"
	"fmt"
	_ "os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	batchSize  = 80         // The number of rows loaded per batch.
	finderPage = "*finder*" // The name of the Finder page.
)

var (
	app         *tview.Application // The tview application.
	pages       *tview.Pages       // The application pages.
	finderFocus tview.Primitive    // The primitive in the Finder that last had focus.
)

var bucketName string = ""

// Main entry point.
func main() {
	// TODO: Consider using some CLI for creds before we do a pop up
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-southeast-2"), // Replace with your AWS region
		Credentials: credentials.NewStaticCredentials("REDACTED", "REDACTED", ""),
	})
	if err != nil {
		panic(err)
	}
	svc := s3.New(sess)
	input := &s3.ListBucketsInput{}

	res, err := svc.ListBuckets(input)
	if err != nil {
		panic(err)
	}

	// Start the application.
	app = tview.NewApplication()

	// Create the basic objects.
	buckets := tview.NewList().ShowSecondaryText(false)
	for _, val := range res.Buckets {
		buckets.AddItem(*val.Name, "", 0, nil)
	}

	// SetBackgroundColor(tcell.ColorDefault)
	// list objects
	buckets.SetBorder(true).SetTitle("Buckets <Ctrl-b>").SetBorderColor(tcell.ColorGreen)
	preview := tview.NewTextView().SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	preview.SetBorder(true).SetTitle("Preview <Ctrl-p>").SetBorderColor(tcell.ColorRed)
	files := tview.NewList()
	files.ShowSecondaryText(false).
		SetDoneFunc(func() {
			files.Clear()
			preview.Clear()
			app.SetFocus(buckets)
		})
	files.SetBorder(true).SetTitle("Files <Ctrl-f>").SetBorderColor(tcell.ColorRed)
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
		files.SetBorderColor(tcell.ColorGreen)
		buckets.SetBorderColor(tcell.ColorRed)
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

	// // Create a footer TextView
	// footer := tview.NewTextView().
	// 	SetText("Press Ctrl-b for Buckets, Ctrl-f for Files, Ctrl-p for Preview").
	// 	SetTextAlign(tview.AlignCenter).
	// 	SetTextColor(tcell.ColorWhite).
	// 	SetBackgroundColor(tcell.ColorBlack)

	// Create the layout.
	flex := tview.NewFlex().
		AddItem(buckets, 0, 1, true).
		AddItem(files, 0, 1, false).
		AddItem(preview, 0, 1, false)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlB:
			app.SetFocus(buckets)
			buckets.SetBorderColor(tcell.ColorGreen)
			files.SetBorderColor(tcell.ColorRed)
			preview.SetBorderColor(tcell.ColorRed)
		case tcell.KeyCtrlF:
			app.SetFocus(files)
			files.SetBorderColor(tcell.ColorGreen)
			buckets.SetBorderColor(tcell.ColorRed)
			preview.SetBorderColor(tcell.ColorRed)
		case tcell.KeyCtrlP:
			app.SetFocus(preview)
			preview.SetBorderColor(tcell.ColorGreen)
			buckets.SetBorderColor(tcell.ColorRed)
			files.SetBorderColor(tcell.ColorRed)
		case tcell.KeyEscape:
			app.Stop()
		}
		return event
	})

	// TODO: figure out how to change colours based on click events
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
