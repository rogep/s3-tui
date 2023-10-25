package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	app          *tview.Application
	bucketName   string
	envName      string
	currentFocus string // allows refocusing when exiting forms/new app state
	selectedFile string
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

	// Add descriptions for positional arguments
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
	// TODO: Consider using some CLI for creds before we do a pop up
	// with no flags set, the application will look at the default profile
	envPtr := flag.Bool("E", false, "Use AWS credentials from environment variables")
	credPtr := flag.Bool("c", false, "Use ephemeral AWS credentials from positional arguments")
	profilePtr := flag.String("p", "default", "Credential profile to select from .aws/credentials. Defaults to \"Default\", or the first found, if no flags are provided.")
	flag.Usage = usage
	flag.Parse()
	fmt.Println(os.Environ())

	var cfg aws.Config

	if (len(flag.Args()) > 3 || len(flag.Args()) == 1) && *envPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) > 1 && !*envPtr {
		fmt.Println("Positional arguments can only be AWS Access key, secret access key and SSO and require the -E flag")
		flag.Usage()
		os.Exit(1)
	} else if len(flag.Args()) == 2 && *credPtr {
		cfg, _ = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("ap-southeast-2"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(flag.Args()[0], flag.Args()[1], "")),
		)
		envName = "cli"
	} else if len(flag.Args()) == 3 && *credPtr {
		cfg, _ = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("ap-southeast-2"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(flag.Args()[0], flag.Args()[1], flag.Args()[2])),
		)
		envName = "cli"
	} else if *envPtr {
		// TODO: remove SSO support -- i dont even use it when i use s3
		awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		awsSSOKey := os.Getenv("AWS_SSO_SOMETHING")
		cfg, _ = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("ap-southeast-2"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKey, awsSecretAccessKey, awsSSOKey)))
		envName = "Environment Variables"
	} else if *profilePtr != "" {
		creds := getAWSCredentialProfiles()
		found := false
		var profileNames []string
		for _, cred := range creds {
			profileNames = append(profileNames, cred.name)
			if cred.name == *profilePtr {
				cfg, _ = config.LoadDefaultConfig(context.TODO(),
					config.WithRegion("ap-southeast-2"),
					config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cred.accessKey, cred.secretAccessKey, cred.sso)))
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
		cfg, _ = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion("ap-southeast-2"),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(creds[0].accessKey, creds[0].secretAccessKey, creds[0].sso)))
		envName = creds[0].name
	}

	s3Client := s3.NewFromConfig(cfg)
	input := &s3.ListBucketsInput{}

	res, err := s3Client.ListBuckets(context.TODO(), input)
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
			MaxKeys: int32(1000),
		}

		result, err := s3Client.ListObjectsV2(context.TODO(), input2)
		if err != nil {
			panic(err)
		}
		files.Clear()
		preview.Clear()
		app.SetFocus(files)
		files.SetBorderColor(tcell.ColorYellow)
		buckets.SetBorderColor(tcell.ColorWhite)
		for _, val := range result.Contents {
			if val.Size == 0 {
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
			_, err := s3Client.DeleteObject(context.TODO(), input)
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
				MaxKeys: int32(1000),
			}

			result, err := s3Client.ListObjectsV2(context.TODO(), input2)
			if err != nil {
				panic(err)
			}
			files.Clear()
			preview.Clear()
			app.SetFocus(files)
			files.SetBorderColor(tcell.ColorYellow)
			buckets.SetBorderColor(tcell.ColorWhite)
			for _, val := range result.Contents {
				if val.Size == 0 {
					continue
				}
				files.AddItem(*val.Key, "", 0, nil)
			}
			// TODO: remove key in rename
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

					_, err := s3Client.CopyObject(context.TODO(), input)
					if err != nil {
						// if aerr, ok := err.(awserr.Error); ok {
						// 	switch aerr.Code() {
						// 	case s3.ErrCodeObjectNotInActiveTierError:
						// 		fmt.Println(s3.ErrCodeObjectNotInActiveTierError, aerr.Error())
						// 	default:
						// 		fmt.Println(aerr.Error())
						// 	}
						// } else {
						// 	fmt.Println(err.Error())
						// }
						return
					}
					// DELETE
					deleteInput := &s3.DeleteObjectInput{
						Bucket: aws.String(bucketName),
						Key:    aws.String(selectedKey),
					}

					_, err = s3Client.DeleteObject(context.TODO(), deleteInput)
					if err != nil {
						if aerr, ok := err.(awserr.Error); ok {
							switch aerr.Code() {
							default:
								fmt.Println(aerr.Error())
							}
						} else {
							fmt.Println(err.Error())
						}
						return
					}
					// UPDATE Files view
					input2 := &s3.ListObjectsV2Input{
						Bucket:  aws.String(bucketName),
						MaxKeys: int32(1000),
					}

					result, err := s3Client.ListObjectsV2(context.TODO(), input2)
					if err != nil {
						panic(err)
					}
					files.Clear()
					preview.Clear()
					app.SetFocus(files)
					files.SetBorderColor(tcell.ColorYellow)
					buckets.SetBorderColor(tcell.ColorWhite)
					for _, val := range result.Contents {
						if val.Size == 0 {
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
		output, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
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

		case tcell.KeyCtrlT:
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
					spinTitle(app, buckets, "Creating bucket", func() {
						input := &s3.CreateBucketInput{
							Bucket: aws.String(bucketInput.GetText()),
						}

						_, err := s3Client.CreateBucket(context.TODO(), input)
						if err != nil {
							panic("lame")
						}
						// if err != nil {
						// 	// s3 buckets can only have 63 chars
						// 	nameLen := len(bucketInput.GetText())
						// 	// this is technically redundent now as field length is capped at 54
						// 	if nameLen+8 <= 63 {
						// 		hash, err := generateRandomString(8)
						// 		if err != nil {
						// 			panic(err)
						// 		}
						// 		uniqueBucketName := bucketInput.GetText() + "-" + hash
						// 		input2 := &s3.CreateBucketInput{
						// 			Bucket: aws.String(uniqueBucketName),
						// 		}
						// 		_, err = svc.CreateBucket(input2)
						// 		if err != nil {
						// 			fmt.Println(input2)
						// 			panic(err)
						// 		}
						// 	}
						// 	return
						// }
					})
					res, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
					if err != nil {
						panic(err)
					}

					buckets.Clear()
					buckets.SetTitle("Buckets <Ctrl+b")
					for _, val := range res.Buckets {
						buckets.AddItem(*val.Name, "", 0, nil)
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

					app.SetRoot(grid, true).SetFocus(buckets)
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
