# Configuring mysql-backup

`mysql-backup` can be configured using one or more of:

* environment variables - container image and binary
* a configuration file - container image and binary
* CLI flags - binary

In all cases, the command line flag option takes precedence over the environment variable which takes
precedence over the config file option.

The environment variables, CLI flag options and config file options are similar, but not exactly the same,
due to variances in how the various are structured. As a general rule:

* Environment variables are all uppercase, with words separated by underscores, and most start with `DB_DUMP`. For example, `DB_DUMP_FREQ=60`.
* CLI flags are all lowercase, with words separated by hyphens. Since the CLI has sub-commands, the `dump-` and `restore-` are unnecessary. For example, `mysql-backup dump --frequency=60` or `mysql-backup restore --target=/foo/file.gz`.

For example, the following are equivalent.

Set dump frequency to 60 minutes:

* Environment variable: `DB_DUMP_FREQ=60`
* CLI flag: `mysql-backup dump --frequency=60`
* Config file:
```yaml
dump:
  frequency: 60
```

Set the dump target to the directory `/db`:

* Environment variable: `DB_DUMP_TARGET=/db`
* CLI flag: `mysql-backup dump --target=/db`
* Config file:
```yaml
dump:
  target: /db
```

**Security Notices**

If using environment variables with any credentials in a container, you should consider the [use of `--env-file=`](https://docs.docker.com/engine/reference/commandline/run/#set-environment-variables-e-env-env-file), [docker secrets](https://docs.docker.com/engine/swarm/secrets/) to keep your secrets out of your shell history

If using CLI flags with any credentials, you should consider using a config file instead of directly
placing credentials in the flags, where they may be kept in shell history.

## Configuration Options

The following are the environment variables, CLI flags and configuration file options for a backup or a restore.

| Purpose | Backup / Restore | CLI Flag | Env Var | Config Key | Default |
| --- | --- | --- | --- | --- | --- |
| hostname to connect to database. Required. | BR | `server` | `DB_SERVER` | `server` |  |
| port to use to connect to database. Optional. | BR | `port` | `DB_PORT` | `port` | 3306 |
| username for the database | BR | `user` | `DB_USER` | `user` |  |
| password for the database | BR | `pass` | `DB_PASS` | `pass` |  |
| names of databases to dump, comma-separated | B | `include` | `DB_NAMES` | `include` | all databases in the server |
| names of databases to exclude from the dump | B | `exclude` | `DB_NAMES_EXCLUDE` | `exclude` |  |
| do not include `USE <database>;` statement in the dump | B | `single-database` | `SINGLE_DATABASE` | `single-database` | `false` |
| restore to a specific database | R | `restore --database` | `RESTORE_DATABASE` | `restore.database` |  |
| how often to do a dump, in minutes | B | `dump --frequency` | `DB_DUMP_FREQ` | `dump.frequency` | `1440` (in minutes), i.e. once per day |
| what time to do the first dump | B | `dump --begin` | `DB_DUMP_BEGIN` | `dump.-begin` | `0`, i.e. immediately |
| cron schedule for dumps | B | `dump --cron` | `DB_DUMP_CRON` | `dump.cron` |  |
| run the backup a single time and exit | B | `dump --once` | `RUN_ONCE` | `dump.once` | `false` |
| enable debug logging | BR | `debug` | `DEBUG` | `debug` | `false` |
| where to put the dump file; see below | B | `dump --target` | `DB_DUMP_TARGET` | `dump.target` |  |
| path to the actual restore file; see below | R | `restore --target` | `DB_RESTORE_TARGET` | `restore.target` |  |
| replace any `:` in the dump filename with `-` | B | `dump --safechars` | `DB_DUMP_SAFECHARS` | `dump.safechars` | `false` |
| AWS access key ID | BR | `aws-access-key-id` | `AWS_ACCESS_KEY_ID` | `aws-access-key-id` |  |
| AWS secret access key | BR | `aws-secret-access-key` | `AWS_SECRET_ACCESS_KEY` | `aws-secret-access-key` |  |
| AWS default region | BR | `aws-default-region` | `AWS_DEFAULT_REGION` | `aws-default-region` |  |
| alternative endpoint URL for S3-interoperable systems | BR | `aws-endpoint-url` | `AWS_ENDPOINT_URL` | `aws-endpoint-url` |  |
| SMB username. May also be specified in `DB_DUMP_TARGET` or `DB_RESTORE_TARGET` with an `smb://` url. | BR | `smb-user` | `SMB_USER` | `smb-user` |  |
| SMB password. May also be specified in `DB_DUMP_TARGET` or `DB_RESTORE_TARGET` with an `smb://` url. | BR | `smb-pass` | `SMB_PASS` | `smb-pass` |  |
| compression to use, one of: `bzip2`, `gzip` | B | `compression` | `COMPRESSION` | `compression` | `gzip` |
| use separate files per schema or a single file in the tar output | B | `dump --by-schema` | `DB_DUMP_BY_SCHEMA` | `dump.by-schema` | `false` |
| keep permissions for a file target | B | `dump --keep-permissions` | `DB_DUMP_KEEP_PERMISSIONS` | `dump.keep-permissions` | `true` |
| when in container, run the dump or restore with `nice`/`ionice` | BR | `` | `NICE` | `` | `false` |
| tmp directory to be used during backup creation and other operations | BR | `tmp` | `TMP_PATH` | `tmp` | system-defined |
| filename to save the target backup file | B | `dump --filename-pattern` | `DB_DUMP_FILENAME_PATTERN` | `dump.filename-pattern` |  |
| directory with scripts to execute before backup | B | `dump --pre-backup-scripts` | `DB_DUMP_PRE_BACKUP_SCRIPTS` | `dump.pre-backup-scripts` | in container, `/scripts.d/pre-backup/` |
| directory with scripts to execute after backup | B | `dump --post-backup-scripts` | `DB_DUMP_POST_BACKUP_SCRIPTS` | `dump.post-backup-scripts` | in container, `/scripts.d/post-backup/` |
| directory with scripts to execute before restore | R | `restore --pre-restore-scripts` | `DB_DUMP_PRE_RESTORE_SCRIPTS` | `dump.pre-restore-scripts` | in container, `/scripts.d/pre-restore/` |
| directory with scripts to execute after restore | R | `restore --post-restore-scripts` | `DB_DUMP_POST_RESTORE_SCRIPTS` | `dump.post-restore-scripts` | in container, `/scripts.d/post-restore/` |


## Unsupported Options

Unsupported options from the old version of `mysql-backup`:

* `MYSQLDUMP_OPTS`: A string of options to pass to `mysqldump`, e.g. `MYSQLDUMP_OPTS="--opt abc --param def --max_allowed_packet=123455678"` will run `mysqldump --opt abc --param def --max_allowed_packet=123455678`
* `AWS_CLI_OPTS`: Additional arguments to be passed to the `aws` part of the `aws s3 cp` command, click [here](https://docs.aws.amazon.com/cli/latest/reference/#options) for a list. _Be careful_, as you can break something!
* `AWS_CLI_S3_CP_OPTS`: Additional arguments to be passed to the `s3 cp` part of the `aws s3 cp` command, click [here](https://docs.aws.amazon.com/cli/latest/reference/s3/cp.html#options) for a list. If you are using AWS KMS, `sse`, `sse-kms-key-id`, etc., may be of interest.

We are working to bring these to `mysql-dump` v1.