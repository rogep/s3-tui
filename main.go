package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

func main() {
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	awsConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	awsConfig := aws.Config{
		Region:      aws.String("ap-southeast-2"),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretAccessKey, ""),
	}

	client := s3.NewFromConfig(awsConfig)

	params := &s3.ListObjectsV2Input{
		Bucket: aws.String("sagemaker-essential-energy-vegetation-management"),
	}

	paginator := s3.NewListObjectsV2Paginator(client, params)

	pageNum := 0
	for paginator.HasMorePages() && pageNum < 3 {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			log.Printf("error: %v", err)
			return
		}
		for _, value := range output.Contents {
			fmt.Println(*value.Key)
		}
		pageNum++
	}
}
