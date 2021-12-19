package cmd

import (
	c "docker4ssh/config"
	"docker4ssh/docker"
	"docker4ssh/validate"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var cli *client.Client

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate docker4ssh specific files (config / profile files)",

	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		cli, err = docker.InitCli()
		return err
	},
}

var validateStrictFlag bool

var validateConfigCmd = &cobra.Command{
	Use:   "config [files]",
	Short: "Validate a docker4ssh config file",

	RunE: func(cmd *cobra.Command, args []string) error {
		return validateConfig(args)
	},
}

var validateConfigFileFlag string

var validateProfileCmd = &cobra.Command{
	Use:   "profile [files]",
	Short: "Validate docker4ssh profile files",

	RunE: func(cmd *cobra.Command, args []string) error {
		return validateProfile(args)
	},
}

func validateConfig(args []string) error {
	config, err := c.LoadConfig(validateConfigFileFlag, false)
	if err != nil {
		return err
	}

	validator := validate.NewConfigValidator(cli, validateStrictFlag, config)

	var result *validate.ValidatorResult
	if len(args) == 0 {
		result = validator.Validate()
	} else {
		var validateFuncs []func() *validate.ValidatorResult
		for _, arg := range args {
			switch strings.ToLower(arg) {
			case "profile":
				validateFuncs = append(validateFuncs, validator.ValidateProfile)
			case "api":
				validateFuncs = append(validateFuncs, validator.ValidateAPI)
			case "ssh":
				validateFuncs = append(validateFuncs, validator.ValidateSSH)
			case "database":
				validateFuncs = append(validateFuncs, validator.ValidateDatabase)
			case "network":
				validateFuncs = append(validateFuncs, validator.ValidateNetwork)
			case "logging":
				validateFuncs = append(validateFuncs, validator.ValidateLogging)
			default:
				return fmt.Errorf("'%s' is not a valid config section", arg)
			}
		}

		var errors []*validate.ValidateError
		for _, validateFunc := range validateFuncs {
			errors = append(errors, validateFunc().Errors...)
		}

		result = &validate.ValidatorResult{
			Strict: validateStrictFlag,
			Errors: errors,
		}
	}

	fmt.Println(result.String())

	if len(result.Errors) > 0 {
		os.Exit(1)
	}

	return nil
}

func validateProfile(args []string) error {
	var files []string

	if len(args) == 0 {
		args = append(args, "/etc/docker4ssh/profile")
	}
	for _, arg := range args {
		stat, err := os.Stat(arg)
		if os.IsNotExist(err) {
			return fmt.Errorf("file %s does not exist: %v", arg, err)
		}
		if stat.IsDir() {
			dir, err := os.ReadDir(arg)
			if err != nil {
				return fmt.Errorf("failed to read directory %s: %v", arg, err)
			}
			for _, file := range dir {
				path, err := filepath.Abs(file.Name())
				if err != nil {
					return err
				}
				files = append(files, path)
			}
		}
	}

	var profiles c.Profiles
	for _, file := range files {
		p, err := c.LoadProfileFile(file, c.HardcodedPreProfile())
		if err != nil {
			return err
		}
		profiles = append(profiles, p...)
	}

	var errors []*validate.ValidateError
	for _, profile := range profiles {
		errors = append(errors, validate.NewProfileValidator(cli, validateStrictFlag, profile).Validate().Errors...)
	}

	result := validate.ValidatorResult{
		Strict: validateStrictFlag,
		Errors: errors,
	}

	fmt.Println(result.String())

	return nil
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.PersistentFlags().BoolVarP(&validateStrictFlag, "strict", "s", false, "If the check should be strict")

	validateCmd.AddCommand(validateConfigCmd)
	validateConfigCmd.Flags().StringVarP(&validateConfigFileFlag, "file", "f", "/etc/docker4ssh/docker4ssh.conf", "Specify a file to check")

	validateCmd.AddCommand(validateProfileCmd)
}
