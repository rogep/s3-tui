package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app          *tview.Application
	bucketName   string
	envName      string
	currentFocus string // allows refocusing when exiting forms/new app state
)

type awsCreds struct {
	name            string
	accessKey       string
	secretAccessKey string
	sso             string
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
		spinners := []string{"/", "|", "\\", "-", "/"}
		var i int
		for {
			select {
			case _ = <-done:
				app.QueueUpdateDraw(func() {
					box.SetTitle(originalTitle) // Restore original title
				})
				return
			case <-time.After(100 * time.Millisecond):
				spin := i % len(spinners)
				app.QueueUpdateDraw(func() {
					box.SetTitle(title + spinners[spin])
				})
				i++
			}
		}
	}()
}

const (
	chars = "0123456789abcdefghijklmnopqrstuvwxyz"
)

func generateRandomString(length int) (string, error) {
	var result string
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result += string(chars[num.Int64()])
	}
	return result, nil
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()

	fmt.Println("\nPositional Arguments:")
	fmt.Println("  arg1        AWS Access Key ID")
	fmt.Println("  arg2        AWS Secret Access Key")
	fmt.Println("  arg3        AWS SSO (Optional)")
}

// TODO: have all functions return (type, error)
func getAWSCredentialProfiles() []awsCreds {
	awsCredentialsFile := os.Getenv("HOME") + "/.aws/credentials"
	file, err := os.Open(awsCredentialsFile)
	if err != nil {
		fmt.Println("Error opening AWS credentials file:", err)
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var credStruct awsCreds
	var profiles []awsCreds

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			credStruct = awsCreds{}
			credStruct.name = profile
		} else if strings.HasPrefix(line, "aws_access_key_id") {
			credStruct.accessKey = strings.Split(line, " ")[2]
		} else if strings.HasPrefix(line, "aws_secret_access_key") {
			credStruct.secretAccessKey = strings.Split(line, " ")[2]
		} else if strings.HasPrefix(line, "sso") {
			credStruct.sso = strings.Split(line, " ")[2]
		} else if line == "\n" || line == "" {
			if credStruct != (awsCreds{}) {
				profiles = append(profiles, credStruct)
				credStruct = awsCreds{}
			}
		}
	}
	// handle EOF case
	if credStruct != (awsCreds{}) {
		profiles = append(profiles, credStruct)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading AWS credentials file:", err)
		panic(err)
	}
	return profiles
}

func main() {
	envPtr := flag.Bool("e", false, "Use AWS credentials from environment variables")
	credPtr := flag.Bool("c", false, "Use ephemeral AWS credentials from positional arguments")
	profilePtr := flag.String("p", "default", "Credential profile to select from .aws/credentials. Defaults to \"Default\", or the first found, if no flags are provided.")
	flag.Usage = usage
	flag.Parse()

	var awsConfig aws.Config

	if (len(flag.Args()) > 3 || len(flag.Args()) < 2) && *credPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) > 1 && !*credPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) == 2 && *credPtr {
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(flag.Args()[0], flag.Args()[1], ""),
		}
		envName = "cli"
	} else if len(flag.Args()) == 3 && *credPtr {
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(flag.Args()[0], flag.Args()[1], flag.Args()[2]),
		}
		envName = "cli"
	} else if *envPtr {
		// TODO: remove SSO support -- i dont even use it when i use s3
		awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		awsSSOKey := os.Getenv("AWS_SSO_SOMETHING")
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretAccessKey, awsSSOKey),
		}
		envName = "Environment Variables"
	} else if *profilePtr != "" {
		creds := getAWSCredentialProfiles()
		found := false
		var profileNames []string
		for _, cred := range creds {
			profileNames = append(profileNames, cred.name)
			if cred.name == *profilePtr {
				awsConfig = aws.Config{
					Region:      aws.String("ap-southeast-2"),
					Credentials: credentials.NewStaticCredentials(cred.accessKey, cred.secretAccessKey, cred.sso),
				}
				envName = cred.name

				found = true
				break
			}
			if !found {
				panic(fmt.Sprintf("Profile: %s not a valid profile. Found: %s", *profilePtr, profileNames))
			}
		}
	} else {
		creds := getAWSCredentialProfiles()
		awsConfig = aws.Config{
			Region:      aws.String("ap-southeast-2"),
			Credentials: credentials.NewStaticCredentials(creds[0].accessKey, creds[0].secretAccessKey, creds[0].sso),
		}
		envName = creds[0].name
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
	files.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		selectedItemIndex := files.GetCurrentItem()
		selectedKey, _ := files.GetItemText(selectedItemIndex)
		switch event.Key() {
		case tcell.KeyCtrlD:
			input := &s3.DeleteObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(selectedKey),
			}
			_, err := svc.DeleteObject(input)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					default:
						fmt.Println(aerr.Error())
					}
				} else {
					// Print the error, cast err to awserr.Error to get the Code and
					// Message from an error.
					fmt.Println(err.Error())
				}
			}
			// need to refresh the bucket view
			input2 := &s3.ListObjectsV2Input{
				Bucket:  aws.String(bucketName),
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
		case tcell.KeyCtrlR:
			trimmedKey := strings.Split(selectedKey, "/")
			renameInput := tview.NewInputField().
				SetLabel(fmt.Sprintf("Rename %s ", trimmedKey[len(trimmedKey)-1])).
				SetFieldWidth(100)

			grid := tview.NewGrid().
				SetRows(1, 0, 1).
				SetColumns(50, 50, 0).
				SetBorders(false).
				AddItem(tview.NewTextView().
					SetTextAlign(tview.AlignLeft).
					SetDynamicColors(true).
					SetText(""), 0, 0, 1, 3, 0, 0, false).
				AddItem(renameInput, 2, 0, 1, 3, 0, 0, false)

			// Add items to the grid
			grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
				AddItem(files, 1, 1, 1, 1, 0, 100, false).
				AddItem(preview, 1, 2, 1, 1, 0, 100, false)

			app.SetRoot(grid, true).SetFocus(renameInput)

			renameInput.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					// A rename is a copy + delete + refresh list
					// COPY
					sourceKey := "/" + bucketName + "/" + selectedKey
					input := &s3.CopyObjectInput{
						Bucket:     aws.String(bucketName),
						CopySource: aws.String(sourceKey),
						Key:        aws.String(renameInput.GetText()),
					}

					_, err := svc.CopyObject(input)
					if err != nil {
						if aerr, ok := err.(awserr.Error); ok {
							switch aerr.Code() {
							case s3.ErrCodeObjectNotInActiveTierError:
								fmt.Println(s3.ErrCodeObjectNotInActiveTierError, aerr.Error())
							default:
								fmt.Println(aerr.Error())
							}
						} else {
							// Print the error, cast err to awserr.Error to get the Code and
							// Message from an error.
							fmt.Println(err.Error())
						}
						return
					}
					// DELETE
					deleteInput := &s3.DeleteObjectInput{
						Bucket: aws.String(bucketName),
						Key:    aws.String(selectedKey),
					}

					_, err = svc.DeleteObject(deleteInput)
					if err != nil {
						if aerr, ok := err.(awserr.Error); ok {
							switch aerr.Code() {
							default:
								fmt.Println(aerr.Error())
							}
						} else {
							// Print the error, cast err to awserr.Error to get the Code and
							// Message from an error.
							fmt.Println(err.Error())
						}
						return
					}
					// UPDATE Files view
					input2 := &s3.ListObjectsV2Input{
						Bucket:  aws.String(bucketName),
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

					grid := tview.NewGrid().
						SetRows(1, 0, 1).
						SetColumns(50, 50, 0).
						SetBorders(false).
						AddItem(tview.NewTextView().
							SetTextAlign(tview.AlignLeft).
							SetDynamicColors(true).
							SetText(""), 0, 0, 1, 3, 0, 0, false).
						AddItem(tview.NewTextView().
							SetTextAlign(tview.AlignLeft).
							SetDynamicColors(true).SetText(fmt.Sprintf("Credentials: [yellow]%s[white] - Shortcuts: ([green]/[white])search | ([green]ESC[white])ape | <[green]Ctrl+[white]> ([green]c[white])reate bucket | ([green]a[white])dd Credentials | ([green]d[white])elete | ([green]r[white])ename | ([green]u[white])pload | ([green]s[white])wap credentials", envName)), 2, 0, 1, 3, 0, 0, false)

					// Add items to the grid
					grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
						AddItem(files, 1, 1, 1, 1, 0, 100, false).
						AddItem(preview, 1, 2, 1, 1, 0, 100, false)

					app.SetRoot(grid, true).SetFocus(files)
				}
			})

		}

		return event
	})
	files.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selectedKey := mainText
		output, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(selectedKey),
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
		AddItem(newPrimitive(fmt.Sprintf("Credentials: [yellow]%s[white] - Shortcuts: ([green]/[white])search | ([green]ESC[white])ape | <[green]Ctrl+[white]> ([green]c[white])reate bucket | ([green]a[white])dd Credentials | ([green]d[white])elete | ([green]r[white])ename | ([green]u[white])pload | ([green]s[white])wap credentials", envName)), 2, 0, 1, 3, 0, 0, false)

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
			currentFocus = "preview"
		case tcell.KeyCtrlA:
			form := tview.NewForm().
				AddInputField("Name", "", 40, nil, nil).
				AddInputField("Access Key", "", 40, nil, nil).
				AddPasswordField("Secret Access Key", "", 40, '*', nil).
				AddInputField("SSO (Optional)", "", 40, nil, nil).
				AddTextView("Note", "Credentials will be stored inside\n.aws/credentials.\nCtrl+Shift+v to paste", 40, 3, true, false)

			form.AddButton("Save", func() {
				// TODO: Probably make this a function
				profileName := form.GetFormItem(0).(*tview.InputField).GetText()
				accessKeyInput := form.GetFormItem(1).(*tview.InputField).GetText()
				secretAccessKeyInput := form.GetFormItem(2).(*tview.InputField).GetText()

				if profileName == "" || accessKeyInput == "" || secretAccessKeyInput == "" {
					app.SetRoot(grid, true).EnableMouse(false).Run()
				}

				awsCredentialsFile := os.Getenv("HOME") + "/.aws/credentials"
				file, err := os.OpenFile(awsCredentialsFile, os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					panic("This is not good")
				}
				defer file.Close()

				writer := bufio.NewWriter(file)
				_, err = fmt.Fprintf(writer, "\n[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n", profileName, accessKeyInput, secretAccessKeyInput)
				if err != nil {
					panic("This is not good either")
				}
				writer.Flush()

				app.SetRoot(grid, true).EnableMouse(false).Run()
				switch currentFocus {
				case "buckets":
					app.SetFocus(buckets)
				case "preview":
					app.SetFocus(preview)
				default:
					app.SetFocus(files)
				}
			}).
				AddButton("Cancel", func() {
					switch currentFocus {
					case "buckets":
						app.SetRoot(grid, true).SetFocus(buckets).EnableMouse(false).Run()
					case "preview":
						app.SetRoot(grid, true).SetFocus(preview).EnableMouse(false).Run()
					default:
						app.SetRoot(grid, true).SetFocus(files).EnableMouse(false).Run()
					}
				})
				// TODO: use app.GetFocus() to handle exit events a lot better

			form.SetBorder(true).SetTitle("AWS Credentials Configuration").SetTitleAlign(tview.AlignLeft)
			app.SetRoot(form, true).EnableMouse(false).Run()

		case tcell.KeyCtrlO:
			bucketInput := tview.NewInputField().
				SetLabel(fmt.Sprintf("Enter bucket name: ")).
				SetFieldWidth(54)

			grid := tview.NewGrid().
				SetRows(1, 0, 1).
				SetColumns(50, 50, 0).
				SetBorders(false).
				AddItem(tview.NewTextView().
					SetTextAlign(tview.AlignLeft).
					SetDynamicColors(true).
					SetText(""), 0, 0, 1, 3, 0, 0, false).
				AddItem(bucketInput, 2, 0, 1, 3, 0, 0, false)

			// Add items to the grid
			grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
				AddItem(files, 1, 1, 1, 1, 0, 100, false).
				AddItem(preview, 1, 2, 1, 1, 0, 100, false)

			app.SetRoot(grid, true).SetFocus(bucketInput)

			bucketInput.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					input := &s3.CreateBucketInput{
						Bucket: aws.String(bucketInput.GetText()),
					}

					_, err := svc.CreateBucket(input)
					if err != nil {
						// s3 buckets can only have 63 chars
						nameLen := len(bucketInput.GetText())
						// this is technically redundent now as field length is capped at 54
						if nameLen+8 <= 63 {
							hash, err := generateRandomString(8)
							if err != nil {
								panic(err)
							}
							uniqueBucketName := bucketInput.GetText() + "-" + hash
							input2 := &s3.CreateBucketInput{
								Bucket: aws.String(uniqueBucketName),
							}
							_, err = svc.CreateBucket(input2)
							if err != nil {
								fmt.Println(input2)
								panic(err)
							}
						}
						return
					}
					res, err := svc.ListBuckets(&s3.ListBucketsInput{})
					if err != nil {
						panic(err)
					}

					buckets.Clear()
					for _, val := range res.Buckets {
						buckets.AddItem(*val.Name, "", 0, nil)
					}

					buckets.SetBorder(true).SetTitle("Buckets <Ctrl+b>")

					grid := tview.NewGrid().
						SetRows(1, 0, 1).
						SetColumns(50, 50, 0).
						SetBorders(false).
						AddItem(tview.NewTextView().
							SetTextAlign(tview.AlignLeft).
							SetDynamicColors(true).
							SetText(""), 0, 0, 1, 3, 0, 0, false).
						AddItem(tview.NewTextView().
							SetTextAlign(tview.AlignLeft).
							SetDynamicColors(true).SetText(fmt.Sprintf("Credentials: [yellow]%s[white] - Shortcuts: ([green]/[white])search | ([green]ESC[white])ape | <[green]Ctrl+[white]> ([green]c[white])reate bucket | ([green]a[white])dd Credentials | ([green]d[white])elete | ([green]r[white])ename | ([green]u[white])pload | ([green]s[white])wap credentials", envName)), 2, 0, 1, 3, 0, 0, false)

					// Add items to the grid
					grid.AddItem(buckets, 1, 0, 1, 1, 0, 100, false).
						AddItem(files, 1, 1, 1, 1, 0, 100, false).
						AddItem(preview, 1, 2, 1, 1, 0, 100, false)

					app.SetRoot(grid, true).SetFocus(files)
				}
			})

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
						switch currentFocus {
						case "buckets":
							app.SetRoot(grid, true).SetFocus(buckets).EnableMouse(false).Run()
						case "preview":
							app.SetRoot(grid, true).SetFocus(preview).EnableMouse(false).Run()
						default:
							app.SetRoot(grid, true).SetFocus(files).EnableMouse(false).Run()
						}
					}
				})
			app.SetRoot(modal, true).EnableMouse(false).Run()

		case tcell.KeyCtrlU:
			var mu sync.Mutex
			mu.Lock()
			defer mu.Unlock()
			spinTitle(app, files, "Uploading... ", func() {
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
