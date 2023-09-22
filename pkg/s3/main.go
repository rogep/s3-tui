package main

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-southeast-2"), // Replace with your AWS region
		Credentials: credentials.NewStaticCredentials("REDACTED", "REDACTED", ""),
	})
	if err != nil {
		panic(err)
	}
	svc := s3.New(sess)

	// Write the contents of S3 Object to the fil// Create a downloader with the session and default options
	// Get object from S3
	output, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String("sagemaker-sdk-test-20220602"),
		Key:    aws.String("example.json"),
		Range:  aws.String("bytes=0-1000"),
	})
	if err != nil {
		fmt.Println("Error getting object ", err)
	}

	// Convert the content to byte slice
	buf := new(bytes.Buffer)
	buf.ReadFrom(output.Body)
	byteContent := buf.Bytes()
	fmt.Printf("file downloaded as byte slice: %s", string(byteContent))
}
