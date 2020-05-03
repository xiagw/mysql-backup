# mysql-backup

Back up mysql databases to... anywhere!

## Overview

mysql-backup is a simple way to do MySQL database backups and restores.

It has the following features:

* dump and restore
* dump to local filesystem or to SMB server
* select database user and password
* connect to any container running on the same system
* select how often to run a dump
* select when to start the first dump, whether time of day or relative to container start time

Please see [CONTRIBUTORS.md](./CONTRIBUTORS.md) for a list of contributors.

## Support

Support is available at the [databack Slack channel](http://databack.slack.com); register [here](https://join.slack.com/t/databack/shared_invite/zt-1cnbo2zfl-0dQS895icOUQy31RAruf7w). We accept issues here and general support questions on Slack.

If you are interested in commercial support, please contact us via Slack above.

## Running `mysql-backup`

`mysql-backup` is available both as a single standalone binary, and as a container image.

## Backup

To run a backup, launch `mysql-backup` - as a container or as a binary - with the correct parameters. 

For example:

````bash
docker run -d --restart=always -e DB_DUMP_FREQ=60 -e DB_DUMP_BEGIN=2330 -e DB_DUMP_TARGET=/local/file/path -e DB_SERVER=my-db-address -v /local/file/path:/db databack/mysql-backup

# or

mysql-backup dump --frequency=60 --begin=2330 --target=/local/file/path --server=my-db-address
````

Or `mysql-backup --config-file=/path/to/config/file.yaml` where `/path/to/config/file.yaml` is a file
with the following contents:

```yaml
server: my-db-address
dump:
  frequency: 60
  begin: 2330
  target: /local/file/path
```

The above will run a dump every 60 minutes, beginning at the next 2330 local time, from the database accessible in the container `my-db-address`.

````bash
docker run -d --restart=always -e DB_USER=user123 -e DB_PASS=pass123 -e DB_DUMP_FREQ=60 -e DB_DUMP_BEGIN=2330 -e DB_DUMP_TARGET=/db -e DB_SERVER=my-db-address -v /local/file/path:/db databack/mysql-backup

# or

mysql-backup dump --user=user123 --pass=pass123 --frequency=60 --begin=2330 --target=/local/file/path --server=my-db-address --port=3306
````

See [backup](./docs/backup.md) for a more detailed description of performing backups.

See [configuration](./docs/configuration.md) for a detailed list of all configuration options.


## Restore

To perform a restore, you simply run the process in reverse. You still connect to a database, but instead of the
dump command, you pass it the restore command. Instead of a dump target, you pass it a restore target.

### Dump Restore

If you wish to run a restore to an existing database, you can use mysql-backup to do a restore.

Examples:

1. Restore from a local file: `docker run -e DB_SERVER=gotodb.example.com -e DB_USER=user123 -e DB_PASS=pass123 -e DB_RESTORE_TARGET=/backup/db_backup_201509271627.gz -v /local/path:/backup databack/mysql-backup`
2. Restore from an SMB file: `docker run -e DB_SERVER=gotodb.example.com -e DB_USER=user123 -e DB_PASS=pass123 -e DB_RESTORE_TARGET=smb://smbserver/share1/backup/db_backup_201509271627.gz databack/mysql-backup`
3. Restore from an S3 file: `docker run -e DB_SERVER=gotodb.example.com -e AWS_ACCESS_KEY_ID=awskeyid -e AWS_SECRET_ACCESS_KEY=secret -e AWS_DEFAULT_REGION=eu-central-1 -e DB_USER=user123 -e DB_PASS=pass123 -e DB_RESTORE_TARGET=s3://bucket/path/db_backup_201509271627.gz databack/mysql-backup`

See [restore](./docs/restore.md) for a more detailed description of performing restores.

See [configuration](./docs/configuration.md) for a detailed list of all configuration options.

## License
Released under the MIT License.
Copyright Avi Deitcher https://github.com/deitch
