package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/databacker/mysql-backup/pkg/compression"
	"github.com/databacker/mysql-backup/pkg/database"
	"github.com/databacker/mysql-backup/pkg/storage/credentials"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type subCommand func() (*cobra.Command, error)

var subCommands = []subCommand{dumpCmd, restoreCmd}

const (
	defaultPort = 3306
)

var (
	dbconn     database.Connection
	creds      credentials.Creds
	compressor compression.Compressor
)

func rootCmd() (*cobra.Command, error) {
	var (
		v   *viper.Viper
		cmd *cobra.Command
	)
	cmd = &cobra.Command{
		Use:   "mysql-backup",
		Short: "backup or restore one or more mysql-compatible databases",
		Long: `Backup or restore one or more mysql-compatible databases.
		In addition to the provided command-line flag options and environment variables,
		when using s3-storage, supports the standard AWS options:
		
		AWS_ACCESS_KEY_ID: AWS Key ID
		AWS_SECRET_ACCESS_KEY: AWS Secret Access Key
		AWS_DEFAULT_REGION: Region in which the bucket resides
		`,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			var err error
			bindFlags(cmd, v)

			if v.GetBool("debug") {
				log.SetLevel(log.DebugLevel)
			}

			dbconn = database.Connection{
				User: v.GetString("user"),
				Pass: v.GetString("pass"),
				Host: v.GetString("server"),
				Port: v.GetInt("port"),
			}
			creds = credentials.Creds{
				AWSEndpoint:    v.GetString("aws-endpoint-url"),
				SMBCredentials: fmt.Sprintf("%s%%%s", v.GetString("smb-user"), v.GetString("smb-pass")),
			}
			compressionAlgo := v.GetString("compression")
			if compressionAlgo != "" {
				compressor, err = compression.GetCompressor(compressionAlgo)
				if err != nil {
					log.Fatalf("failure to get compression '%s': %v", compressionAlgo, err)
				}
			}
		},
	}

	v = viper.New()
	v.SetEnvPrefix("db")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	// server hostname via CLI or env var
	pflags := cmd.PersistentFlags()
	pflags.String("server", "", "hostname for database server")
	if err := cmd.MarkPersistentFlagRequired("server"); err != nil {
		return nil, err
	}

	// base of temporary directory to use
	pflags.String("tmp", os.TempDir(), "temporary directory base for working directory, defaults to OS")

	// server port via CLI or env var or default
	pflags.Int("port", defaultPort, "port for database server")

	// user via CLI or env var
	pflags.String("user", "", "username for database server")

	// pass via CLI or env var
	pflags.String("pass", "", "password for database server")

	// debug via CLI or env var or default
	pflags.Bool("debug", false, "enable debug logging")

	// aws options
	pflags.String("aws-endpoint-url", "", "Specify an alternative endpoint for s3 interoperable systems e.g. Digitalocean; ignored if not using s3.")
	pflags.String("aws-access-key-id", "", "Access Key for s3 and s3 interoperable systems; ignored if not using s3.")
	pflags.String("aws-secret-access-key", "", "Secret Access Key for s3 and s3 interoperable systems; ignored if not using s3.")
	pflags.String("aws-default-region", "", "Region for s3 and s3 interoperable systems; ignored if not using s3.")

	// smb options
	pflags.String("smb-user", "", "SMB username. May also be specified in --target with an smb:// url. If both specified, this variable overrides the value in the URL.")
	pflags.String("smb-pass", "", "SMB username. May also be specified in --target with an smb:// url. If both specified, this variable overrides the value in the URL.")

	for _, subCmd := range subCommands {
		if sc, err := subCmd(); err != nil {
			return nil, err
		} else {
			cmd.AddCommand(sc)
		}
	}

	return cmd, nil
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Determine the naming convention of the flags when represented in the config file
		configName := f.Name
		_ = v.BindPFlag(configName, f)
		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

// Execute primary function for cobra
func Execute() {
	rootCmd, err := rootCmd()
	if err != nil {
		log.Fatal(err)
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
