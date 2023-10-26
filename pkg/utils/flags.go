package utils

import (
	"flag"
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()

	// Add descriptions for positional arguments
	fmt.Println("\nPositional Arguments:")
	fmt.Println("  arg1        AWS Access Key ID")
	fmt.Println("  arg2        AWS Secret Access Key")
	fmt.Println("  arg3        AWS SSO (Optional)")
}

func ParseFlags() (envPtr *bool, credPtr *bool, profilePtr *string) {
	envPtr = flag.Bool("E", false, "Use AWS credentials from environment variables")
	credPtr = flag.Bool("c", false, "Use ephemeral AWS credentials from positional arguments")
	profilePtr = flag.String("p", "default", "Credential profile to select from .aws/credentials. Defaults to \"Default\", or the first found, if no flags are provided.")
	flag.Usage = usage
	flag.Parse()
	return
}
