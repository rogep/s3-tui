package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogep/s3-tui/pkg/s3handler"
)

// type s3Handler struct {
// 	s3Client *s3.Client
// }
//
// func newS3Handler(client s3.Client) *s3Handler {
// 	return &s3Handler{
// 		s3Client: &client,
// 	}
// }
//
// func (s *s3Handler) getFolderNames(b string, d string, p string) ([]string, error) {
// 	params := &s3.ListObjectsV2Input{
// 		Bucket:    aws.String(b),
// 		Delimiter: aws.String(d),
// 		Prefix:    aws.String(p),
// 	}
//
// 	paginator := s3.NewListObjectsV2Paginator(s.s3Client, params)
// 	var folders []string
// 	if p != "" {
// 		folders = append(folders, "..")
// 	}
//
// 	for paginator.HasMorePages() {
// 		output, err := paginator.NextPage(context.TODO())
// 		if err != nil {
// 			fmt.Printf("error: %v", err)
// 			return nil, err
// 		}
// 		for _, value := range output.CommonPrefixes {
// 			key := *value.Prefix
// 			folders = append(folders, key)
// 		}
// 	}
//
// 	return folders, nil
// }
//
// func (s *s3Handler) getKeyNames(b string, d string, p string) ([]string, error) {
// 	params := &s3.ListObjectsV2Input{
// 		Bucket:    aws.String(b),
// 		Delimiter: aws.String(d),
// 		Prefix:    aws.String(p),
// 	}
// 	paginator := s3.NewListObjectsV2Paginator(s.s3Client, params)
// 	var keys []string
//
// 	for paginator.HasMorePages() {
// 		output, err := paginator.NextPage(context.TODO())
// 		if err != nil {
// 			fmt.Printf("error: %v", err)
// 			return nil, err
// 		}
// 		for _, value := range output.Contents {
// 			key := *value.Key
// 			if key[len(key)-1:] == "/" || key == "" {
// 				continue
// 			}
// 			keys = append(keys, key)
// 		}
// 	}
// 	return keys, nil
// }

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	s := s3handler.NewS3Handler(*client)
	// NB: you must have a / at the end of your prefix otherwise it will only return your prefix as a folder. Not what we want!!
	folders, _ := s.GetFolderNames("consilium-ml-projects-efs-backups", "/", "boart-longyear-downhole-ml/Final_PLSR_Models/")
	keys, _ := s.GetKeyNames("consilium-ml-projects-efs-backups", "/", "boart-longyear-downhole-ml/Final_PLSR_Models/")
	combined := append(folders, keys...)
	fmt.Println(combined)
}
