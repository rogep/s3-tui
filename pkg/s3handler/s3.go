package s3handler

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Handler struct {
	s3Client *s3.Client
}

func NewS3Handler(client s3.Client) *S3Handler {
	return &S3Handler{
		s3Client: &client,
	}
}

func (s *S3Handler) GetFolderNames(b string, d string, p string) ([]string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(b),
		Delimiter: aws.String(d),
		Prefix:    aws.String(p),
	}

	paginator := s3.NewListObjectsV2Paginator(s.s3Client, params)
	var folders []string
	if p != "" {
		folders = append(folders, "..")
	}

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			fmt.Printf("error: %v", err)
			return nil, err
		}
		for _, value := range output.CommonPrefixes {
			key := *value.Prefix
			folders = append(folders, key)
		}
	}

	return folders, nil
}

func (s *S3Handler) GetKeyNames(b string, d string, p string) ([]string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(b),
		Delimiter: aws.String(d),
		Prefix:    aws.String(p),
	}
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, params)
	var keys []string

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			fmt.Printf("error: %v", err)
			return nil, err
		}
		for _, value := range output.Contents {
			key := *value.Key
			if key[len(key)-1:] == "/" || key == "" {
				continue
			}
			keys = append(keys, key)
		}
	}
	return keys, nil
}
