package cmd

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/databacker/mysql-backup/pkg/core"
)

const (
	frequency          = 1440
	defaultCompression = "gzip"
)

func dumpCmd() (*cobra.Command, error) {
	var v *viper.Viper
	var cmd = &cobra.Command{
		Use:     "dump",
		Aliases: []string{"backup"},
		Short:   "backup a database",
		Long: `Backup a database to a target location, once or on a schedule.
		Can choose to dump all databases, only some by name, or all but excluding some.
		The databases "information_schema", "performance_schema", "sys" and "mysql" are
		excluded by default, unless you explicitly list them.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			bindFlags(cmd, v)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting dump")
			targets := v.GetStringSlice("target")
			if len(targets) == 0 {
				log.Error("must provide at least one target")
				os.Exit(1)
			}
			dumpOpts := core.DumpOptions{
				Targets:           targets,
				Safechars:         v.GetBool("safechars"),
				BySchema:          v.GetBool("byschema"),
				KeepPermissions:   v.GetBool("keep-permissions"),
				DBNames:           v.GetStringSlice("include"),
				DBConn:            dbconn,
				Creds:             creds,
				Compressor:        compressor,
				Exclude:           v.GetStringSlice("exclude"),
				PreBackupScripts:  v.GetString("pre-backup-scripts"),
				PostBackupScripts: v.GetString("post-backup-scripts"),
			}
			timerOpts := core.TimerOptions{
				Once:      v.GetBool("once"),
				Cron:      v.GetString("cron"),
				Begin:     v.GetString("begin"),
				Frequency: v.GetInt("frequency"),
			}
			err := core.TimerDump(dumpOpts, timerOpts)
			if err != nil {
				return err
			}
			log.Info("Backup complete")
			return nil
		},
	}

	v = viper.New()
	v.SetEnvPrefix("db_dump")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	flags := cmd.Flags()
	// target - where the backup is to be saved
	flags.StringSlice("target", []string{}, `full URL target to where the backups should be saved. Should be a directory. Accepts multiple targets. Supports three formats:
Local: If if starts with a "/" character of "file:///", will dump to a local path, which should be volume-mounted.
SMB: If it is a URL of the format smb://hostname/share/path/ then it will connect via SMB.
S3: If it is a URL of the format s3://bucketname/path then it will connect via S3 protocol.`)
	if err := cmd.MarkFlagRequired("target"); err != nil {
		return nil, err
	}

	// include - include of databases to back up
	flags.StringSlice("include", []string{}, "names of databases to dump; empty to do all")

	// exclude
	flags.StringSlice("exclude", []string{}, "databases to exclude from the dump.")

	// single database, do not include `USE database;` in dump
	flags.Bool("single-database", false, "backup for a single database, without `USE <database>;` in the dump.")

	// frequency
	flags.Int("frequency", frequency, "how often to run backups, in minutes")

	// begin
	flags.String("begin", "+0", "What time to do the first dump. Must be in one of two formats: Absolute: HHMM, e.g. `2330` or `0415`; or Relative: +MM, i.e. how many minutes after starting the container, e.g. `+0` (immediate), `+10` (in 10 minutes), or `+90` in an hour and a half")

	// cron
	flags.String("cron", "", "Set the dump schedule using standard [crontab syntax](https://en.wikipedia.org/wiki/Cron), a single line.")

	// once
	flags.Bool("once", false, "Override all other settings and run the dump once immediately and exit. Useful if you use an external scheduler (e.g. as part of an orchestration solution like Cattle or Docker Swarm or [kubernetes cron jobs](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/)) and don't want the container to do the scheduling internally.")

	// safechars
	flags.Bool("safechars", false, "The dump filename usually includes the character `:` in the date, to comply with RFC3339. Some systems and shells don't like that character. If true, will replace all `:` with `-`.")

	// compression
	flags.String("compression", defaultCompression, "Compression to use. Supported are: `gzip`, `bzip2`")

	// by-schema
	flags.Bool("by-schema", false, "Whether to use separate files per schema in the compressed file (`true`), or a single dump file (`false`).")

	// keep permissions
	flags.Bool("keep-permissions", true, "Whether to keep permissions for a file target. By default, `mysql-backup` copies the backup compressed file to the target with `cp -a`. In certain filesystems with certain permissions, this may cause errors. You can disable the `-a` flag by setting this option to false.")

	// source filename pattern
	flags.String("filename-pattern", "db_backup_{{ .now }}.{{ .compression }}", "Pattern to use for filename in target. See documentation.")

	// pre-backup scripts
	flags.String("pre-backup-scripts", "", "Directory wherein any file ending in `.sh` will be run pre-backup.")

	// post-backup scripts
	flags.String("post-backup-scripts", "", "Directory wherein any file ending in `.sh` will be run post-backup but pre-send to target.")

	return cmd, nil
}
