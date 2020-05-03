package cmd

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/databacker/mysql-backup/pkg/core"
)

func restoreCmd() (*cobra.Command, error) {
	var v *viper.Viper
	var cmd = &cobra.Command{
		Use:   "restore",
		Short: "restore a dump",
		Long:  `Restore a database dump from a given location.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			bindFlags(cmd, v)
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Debug("starting restore")
			target := v.GetString("target")
			if err := core.Restore(target, dbconn, creds, compressor); err != nil {
				log.Errorf("error restoring: %v", err)
				os.Exit(1)
			}
			log.Info("Restore complete")
		},
	}
	// target - where the backup is
	v = viper.New()
	v.SetEnvPrefix("db_restore")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	flags := cmd.Flags()
	flags.String("target", "", "full URL target to the backup that you wish to restore")
	if err := cmd.MarkFlagRequired("target"); err != nil {
		return nil, err
	}

	// compression
	flags.String("compression", defaultCompression, "Compression to use. Supported are: `gzip`, `bzip2`")

	// specific database to which to restore
	flags.String("database", "", "Specific database to use to restore. If not specified, the backup file must have the appropriate `USE <database>'` clauses.")

	// pre-restore scripts
	flags.String("pre-restore-scripts", "", "Directory wherein any file ending in `.sh` will be run after retrieving the dump file but pre-restore.")

	// post-restore scripts
	flags.String("post-restore-scripts", "", "Directory wherein any file ending in `.sh` will be run post-restore.")

	return cmd, nil
}
