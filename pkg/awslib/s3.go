package awslib

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogep/s3-tui/pkg/utils"
)

const byteRange string = "bytes=0-1000"

type S3Handler struct {
	s3Client *s3.Client
}

func NewS3Handler(client s3.Client) *S3Handler {
	return &S3Handler{
		s3Client: &client,
	}
}

func (s *S3Handler) GetDirectoryStructure(bucket string, delimiter string, prefix string) ([]string, error) {
	folders, err := s.GetFolderNames(bucket, delimiter, prefix)
	if err != nil {
		return nil, err
	}

	keys, err := s.GetKeyNames(bucket, delimiter, prefix)
	if err != nil {
		return nil, err
	}

	return append(folders, keys...), nil
}

func (s *S3Handler) GetFolderNames(bucket string, delimiter string, prefix string) ([]string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String(delimiter),
		Prefix:    aws.String(prefix),
	}

	paginator := s3.NewListObjectsV2Paginator(s.s3Client, params)
	var folders []string
	if prefix != "" {
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

func (s *S3Handler) GetKeyNames(bucket string, delimiter string, prefix string) ([]string, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String(delimiter),
		Prefix:    aws.String(prefix),
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

func (s *S3Handler) GetBuckets() ([]string, error) {
	input := &s3.ListBucketsInput{}
	res, err := s.s3Client.ListBuckets(context.TODO(), input)
	var buckets []string

	if err != nil {
		return nil, err
	}

	for _, val := range res.Buckets {
		buckets = append(buckets, *val.Name)
	}
	return buckets, nil
}

func (s *S3Handler) CreateBucket(name string, length int) (bool, error) {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(name),
	}

	_, err := s.s3Client.CreateBucket(context.TODO(), input)
	// TODO: handle collision by adding the hex if needed. Need to import utils
	if err != nil {
		hash, hashErr := utils.GenerateRandomString(length)
		if hashErr != nil {
			return false, hashErr
		}
		// S3 buckets cannot exceed 63 chars -- ui caps user input at 54 chars
		uniqueBucketName := name + "-" + hash
		input = &s3.CreateBucketInput{
			Bucket: aws.String(uniqueBucketName),
		}
		_, err = s.s3Client.CreateBucket(context.TODO(), input)
		if err != nil {
			// enter some small brain recursion to generate a new hash
			// inefficient as initial collision will always be hit. but bruh who cares
			return s.CreateBucket(name, length)
		}
		return false, err
	}
	return true, nil
}

// TODO: write utility function that checks the byte slice for non utf-8 chars
// if this is present, display a /// cannot display binary /// message
func (s *S3Handler) PreviewFile(bucket string, key string) ([]byte, error) {
	output, err := s.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(byteRange),
	})
	if err != nil {
		fmt.Println("Error getting object ", err)
	}

	// Convert the content to byte slice
	buf := new(bytes.Buffer)
	buf.ReadFrom(output.Body)
	byteContent := buf.Bytes()
	return byteContent, nil
}
