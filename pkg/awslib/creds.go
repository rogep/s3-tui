package awslib

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func InitCredentials(flag *flag.FlagSet, envPtr *bool, credPtr *bool, profilePtr *string) (aws.Config, string) {
	var cfg aws.Config
	var envName string

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
	return cfg, envName
}

type awsCreds struct {
	name            string
	accessKey       string
	secretAccessKey string
	sso             string
}

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
