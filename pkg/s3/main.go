package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogep/s3-tui/pkg/awslib"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	s := awslib.NewS3Handler(*client)
	// NB: you must have a / at the end of your prefix otherwise it will only return your prefix as a folder. Not what we want!!
	// folders, _ := s.GetFolderNames("consilium-ml-projects-efs-backups", "/", "boart-longyear-downhole-ml/Final_PLSR_Models/")
	// keys, _ := s.GetKeyNames("consilium-ml-projects-efs-backups", "/", "boart-longyear-downhole-ml/Final_PLSR_Models/")
	// combined := append(folders, keys...)
	// fmt.Println(combined)
	directory, _ := s.GetDirectoryStructure("consilium-ml-projects-efs-backups", "/", "boart-longyear-downhole-ml/Final_PLSR_Models/")
	fmt.Println(directory)
}
