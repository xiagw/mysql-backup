# Restoring

Restoring uses the same database, SMB and S3 configuration options as [backup](./docs/backup.md).

The primary difference is the use of restore target, instead of a dump target. This follows the same syntax as
the dump target, but instead of a dump directory, it is the actual restore file.

For example, to restore from a local file:

* Environment variable: `DB_RESTORE_TARGET=/backup/db_backup_201509271627.gz`
* Command line: `restore --target=/backup/db_backup_201509271627.gz`
* Config file:
```yaml
restore:
    target: /backup/db_backup_201509271627.gz`
```

The restore target should be a compressed dump file.

### Restore when using docker-compose
`docker-compose` automagically creates a network when started. `docker run` simply attaches to the bridge network. If you are trying to communicate with a mysql container started by docker-compose, you'll need to specify the network in your command arguments. You can use `docker network ls` to see what network is being used, or you can declare a network in your docker-compose.yml.

#### Example:
`docker run -e DB_SERVER=gotodb.example.com -e DB_USER=user123 -e DB_PASS=pass123 -e DB_RESTORE_TARGET=/backup/db_backup_201509271627.gz -v /local/path:/backup --network="skynet" databack/mysql-backup`

### Using docker (or rancher) secrets
Environment variables used in this image can be passed in files as well. This is useful when you are using docker (or rancher) secrets for storing sensitive information.

As you can set environment variable with `-e ENVIRONMENT_VARIABLE=value`, you can also use `-e ENVIRONMENT_VARIABLE_FILE=/path/to/file`. Contents of that file will be assigned to the environment variable.

**Example:**

```bash
docker run -d \
  -e DB_HOST_FILE=/run/secrets/DB_HOST \
  -e DB_USER_FILE=/run/secrets/DB_USER \
  -e DB_PASS_FILE=/run/secrets/DB_PASS \
  -v /local/file/path:/db \
  databack/mysql-backup
```

### Restore pre and post processing

As with backups pre and post processing, you have pre- and post-restore processing.

This is useful if you need to restore a backup file that includes some files along with the database dump.
For example, to restore a _WordPress_ install, you would uncompress a tarball containing
the db backup and a second tarball with the contents of a WordPress install on
`pre-restore`. Then on `post-restore`, uncompress the WordPress files on the container's web server root directory.

In order to perform pre-restore processing, set the pre-restore processing directory, and `mysql-backup`
will execute any file that ends in `.sh`. For example:

* Environment variable: `DB_DUMP_PRE_RESTORE_SCRIPTS=/scripts.d/pre-restore`
* Command line: `restore --pre-restore-scripts=/scripts.d/pre-restore`
* Config file:
```yaml
restore:
    pre-restore-scripts: /scripts.d/pre-restore
```

When running in a container, these are set automatically to `/scripts.d/pre-restore` and `/scripts.d/post-restore`
respectively.

For an example take a look at the post-backup examples, all variables defined for post-backup scripts are available for pre-processing too. Also don't forget to add the same host volumes for `pre-restore` and `post-restore` directories as described for post-backup processing.

### Restoring to a single or different database

If your dump file does *not* have `USE <database>;` in it, for example, if it was created with
`mysql-backup dump --single-database`, you will want to select a specific database to restore into.

You can do this by telling `mysql-backup restore` which database to restore into. For example,
to restore into database named `foo`:

* Environment variable: `DB_RESTORE_DATABASE=foo`
* Command line: `restore --database=foo`
* Config file:
```yaml
restore:
    database: foo
``` 
