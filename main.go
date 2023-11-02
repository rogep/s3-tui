package main

import (
	"flag"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/rogep/s3-tui/pkg/awslib"
	"github.com/rogep/s3-tui/pkg/gui"
	"github.com/rogep/s3-tui/pkg/utils"
)

func main() {
	envPtr, credPtr, profilePtr := utils.ParseFlags()

	var cfg aws.Config

	cfg, envName := awslib.InitCredentials(flag.CommandLine, envPtr, credPtr, profilePtr)
	s3Client := s3.NewFromConfig(cfg)
	s := awslib.NewS3Handler(*s3Client)
	gui.S3Gui(s, envName)
}
