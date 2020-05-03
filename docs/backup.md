# Backing Up

Backing up is the process of taking backups from your database via `mysql-backup`, and saving the backup file
to a target. That target can be one of:

* local file
* SMB remote file
* S3 bucket

## Instructions and Examples for Backup Configuration Options

### Database Names

By default, all databases in the database server are backed up, and the system databases
named `information_schema`, `performance_schema`, `sys` and `mysql` are excluded.
This only applies if `DB_DUMP_BY_SCHEMA` is set to `true`. For example, if you set `DB_NAMES_EXCLUDE=database1 db2` and `DB_DUMP_BY_SCHEMA=true` then these two databases will not be dumped.

**Dumping just some databases**

* Environment variable: `DB_NAMES=db1,db2,db3`
* CLI flag: `--include=db1 --include=db2 --include=db3`
* Config file:
```yaml
dump:
  include:
  - db1
  - db2
  - db3
```

**Dumping all databases**

* Environment variable: `DB_NAMES=`
* CLI flag: `--include=`
* Config file:
```yaml
dump:
  include:
```

Note that you do not need to set those explicitly; these are the defaults for those settings.

**Dumping all databases except for one**

* Environment variable: `DB_NAMES_EXCLUDE=notme,notyou`
* CLI flag: `--exclude=notme,notyou`
* Config file:
```yaml
dump:
  exclude:
  - notme
  - notyou
```

### Dumping by Schema

### Single Database

By default, the backup assumes you will restore the dump into a database with the same name as the
one that you backed up. This means it will include the `USE <database>;` statement in the dump, so
it will switch to the correct database when you restore the dump.

If you want to restore the dump to a different database, you need to remove the `USE <database>;` statement
from the dump. `mysql-backup` does this for you when you set:

* Environment variable: `SINGLE_DATABASE=true`.
* CLI flag: `--single-database=true`
* Config file:
```yaml
dump:
  single-database: true
```

### Dump File

The backup file itself *always* is a compressed file the following format:

`db_backup_YYYY-MM-DDTHH:mm:ssZ.<compression>`

Where the date is RFC3339 date format, excluding the milliseconds portion.

* YYYY = year in 4 digits
* MM = month number from 01-12
* DD = date for 01-31
* HH = hour from 00-23
* mm = minute from 00-59
* ss = seconds from 00-59
* T = literal character `T`, indicating the separation between date and time portions
* Z = literal character `Z`, indicating that the time provided is UTC, or "Zulu"
* compression = appropriate file ending for selected compression, one of: `gz` (gzip, default); `bz2` (bzip2)

The time used is UTC time at the moment the dump begins.

Notes on format:

* SMB does not allow for `:` in a filename (depending on server options), so they are replaced with the `-` character when writing to SMB.
* Some shells do not handle a `:` in the filename gracefully. Although these usually are legitimate characters as far as the _filesystem_ is concerned, your shell may not like it. To avoid this issue, you can set the "no-colons" options with the "safechars" configuration:

* Environment variable: `DB_DUMP_SAFECHARS=true`
* CLI flag: `dump --safechars=true`
* Config file:
```yaml
dump:
  safechars: true
```

### Dump Target

You set the directory where to put the dump file via configuration. For example, to set it to a local directory
named `/db`:

* Environment variable: `DB_DUMP_TARGET=/db`
* CLI flag: `dump --target=/db`
* Config file:
```yaml
dump:
  target: /db
```

It **must** be a directory.

The value of the variable can be one of three formats, depending on the format of the value:

* Local: If it starts with a `/` character, will dump to a local path. If in a container, you should have it volume-mounted.
* SMB: If it is a URL of the format `smb://hostname/share/path/` then it will connect via SMB.
* S3: If it is a URL of the format `s3://bucketname/path` then it will connect via using the S3 protocol.

In addition, you can send to multiple targets by separating them with a whitespace for the environment variable,
or native multiple options for other configuration options. For example, to send to a local directory and an SMB share:

* Environment variable: `DB_DUMP_TARGET="/db smb://hostname/share/path/"`
* CLI flag: `dump --target=/db --target=smb://hostname/share/path/"`
* Config file:
```yaml
dump:
  target:
  - /db
  - smb://hostname/share/path/
```

#### Local File

If the target starts with `/` or is a `file:///` then it is assumed to be a directory. The file will be written to that
directory.

The target **must** be to a directory, wherein the dump file will be saved, using the naming
convention listed above.

##### Dump File Permissions and Ownership

When using a file target, `mysql-backup` copies the backup compressed file to the target with the equivalent of
`cp -a`. In certain filesystems with certain permissions, this may cause errors. You can disable the
`-a` flag by using the setting:

* Environment variable: `DB_DUMP_KEEP_PERMISSIONS=false`
* CLI flag: `dump --keep-permissions=false`
* Config file:
```yaml
dump:
  keep-permissions: false
```

##### Container Considerations

If running in a container, you will need to ensure that the target is mounted. See
[container considerations](./container_considerations.md).

#### SMB

If you use a URL that begins with `smb://`, for example `smb://host/share/path`, the dump file will be saved
to an SMB server.

The full URL **must** be to a directory on the SMB server, wherein the dump file will be saved, using the naming
convention listed above.

If you need login credentials, you can either use the URL format `smb://user:pass@host/share/path`,
or you can use the SMB user and password options:

* Environment variable: `SMB_USER=user SMB_PASS=pass`
* CLI flag: `--smb-user=user --smb-pass=pass`
* Config file:
```yaml
smb-user: user
smb-pass: pass
```

The explicit credentials in `SMB_USER` and `SMB_PASS` override user and pass values in the URL.

Note that for smb, if the username includes a domain, e.g. your user is `mydom\myuser`, then you should use the smb convention of replacing the '\' with a ';'. In other words `smb://mydom;myuser:pass@host/share/path`

##### S3

If you use a URL that begins with `s3://`, for example `s3://bucket/path`, the dump file will be saved to the S3 bucket.

The full URL **must** be to a directory in the S3 bucket, wherein the dump file will be saved, using the naming
convention listed above.

Note that for s3, you'll need to specify your AWS credentials and default AWS region via the appropriate
settings.

For example, to set the AWS credentials:

* Environment variable: `AWS_ACCESS_KEY_ID=accesskey AWS_SECRET_ACCESS_KEY=secretkey AWS_DEFAULT_REGION=us-east-1`
* CLI flag: `--aws-access-key-id=accesskey --aws-secret-access-key=secretkey --aws-default-region=us-east-1`
* Config file:
```yaml
aws-access-key-id: accesskey
aws-secret-access-key: secretkey
aws-default-region: us-east-1
```

If you are using an s3-interoperable storage system like DigitalOcean you will need to
set the AWS endpoint URL via the AWS endpoint URL setting.

For example, to use Digital Ocean, whose endpoint URL is `${REGION_NAME}.digitaloceanspaces.com`:

* Environment variable: `AWS_ENDPOINT_URL=https://nyc3.digitaloceanspaces.com`
* CLI flag: `--aws-endpoint-url=https://nyc3.digitaloceanspaces.com`
* Config file:
```yaml
aws-endpoint-url: https://nyc3.digitaloceanspaces.com
```
 
 #### Custom backup file name

There may be use-cases where you need to modify the name and path of the backup file when it gets uploaded to the dump target.

For example, if you need the filename not to be `<root-dir>/db_backup_<timestamp>.gz` but perhaps `<root-dir>/<year>/<month>/<day>/mybackup_<timestamp>.gz`.

To do that, configure the environment variable `DB_DUMP_FILENAME_PATTERN` or its CLI flag or config file equivalent.

The content is a string that contains a pattern to be used for the filename. The pattern can contain the following placeholders:

* `{{.now}}` - date of the backup, as included in `{{.dumpfile}}` and given by `date -u +"%Y-%m-%dT%H:%M:%SZ"`
* `{{.year}}`
* `{{.month}}`
* `{{.day}}`
* `{{.hour}}`
* `{{.minute}}`
* `{{.second}}`
* `{{.compression}}` - appropriate extension for the compression used, for example, `.gz` or `.bz2`

**Example run:**

```
mysql-backup dump --source-filename-pattern="db-plus-wordpress_{{.now}}.gz"
```

If the execution time was `20180930151304`, then the file will be named `plus-wordpress_20180930151304.gz`.

### Backup pre and post processing

`mysql-backup` is capable of running arbitrary scripts for pre-backup and post-backup (but pre-upload)
processing. This is useful if you need to include some files along with the database dump, for example,
to backup a _WordPress_ install.

In order to execute those scripts, you deposit them in appropriate dedicated directories and
inform `mysql-backup` about the directories. Any file ending in `.sh` in the directory will be executed.

* When using the binary, set the directories via the environment variable `DB_DUMP_PRE_BACKUP_SCRIPTS` or `DB_DUMP_POST_BACKUP_SCRIPTS`, or their CLI flag or config file equivalents.
* When using the `mysql-backup` container, these are automatically set to the directories `/scripts.d/pre-backup/` and `/scripts.d/post-backup/`, inside the container respectively. It is up to you to mount them.

**Example run binary:**

```bash
mysql-backup dump --pre-backup-scripts=/path/to/pre-backup/scripts --post-backup-scripts=/path/to/post-backup/scripts
```

**Example run container:**

```bash
docker run -d --restart=always -e DB_USER=user123 -e DB_PASS=pass123 -e DB_DUMP_FREQ=60 \
  -e DB_DUMP_BEGIN=2330 -e DB_DUMP_TARGET=/db -e DB_SERVER=my-db-container:db \
  -v /path/to/pre-backup/scripts:/scripts.d/pre-backup \
  -v /path/to/post-backup/scripts:/scripts.d/post-backup \
  -v /local/file/path:/db \
  databack/mysql-backup
```

Or, if you prefer [docker compose](https://docs.docker.com/compose/):

```yml
version: '2.1'
services:
  backup:
    image: databack/mysql-backup
    restart: always
    volumes:
     - /local/file/path:/db
     - /path/to/pre-backup/scripts:/scripts.d/pre-backup
     - /path/to/post-backup/scripts:/scripts.d/post-backup
    env:
     - DB_DUMP_TARGET=/db
     - DB_USER=user123
     - DB_PASS=pass123
     - DB_DUMP_FREQ=60
     - DB_DUMP_BEGIN=2330
     - DB_SERVER=mysql_db
  mysql_db:
    image: mysql
    ....
```

The scripts are _executed_ in the [entrypoint](https://github.com/databack/mysql-backup/blob/master/entrypoint) script, which means it has access to all exported environment variables. The following are available, but we are happy to export more as required (just open an issue or better yet, a pull request):

* `DUMPFILE`: full path in the container to the output file
* `NOW`: date of the backup, as included in `DUMPFILE` and given by `date -u +"%Y-%m-%dT%H:%M:%SZ"`
* `DUMPDIR`: path to the destination directory so for example you can copy a new tarball including some other files along with the sql dump.
* `DEBUG`: To enable debug mode in post-backup scripts.

In addition, all of the environment variables set for the container will be available to the script.

For example, the following script will rename the backup file after the dump is done:

````bash
#!/bin/bash
# Rename backup file.
if [[ -n "$DEBUG" ]]; then
  set -x
fi

if [ -e ${DUMPFILE} ];
then
  now=$(date +"%Y-%m-%d-%H_%M")
  new_name=db_backup-${now}.gz
  old_name=$(basename ${DUMPFILE})
  echo "Renaming backup file from ${old_name} to ${new_name}"
  mv ${DUMPFILE} ${DUMPDIR}/${new_name}
else
  echo "ERROR: Backup file ${DUMPFILE} does not exist!"
fi

````

### Encrypting the Backup

Post-processing gives you options to encrypt the backup using openssl or any other tools. You will need to have it
available on your system. When running in the `mysql-backup` container, the openssl binary is available
to the processing scripts.

The sample [examples/encrypt.sh](./examples/encrypt.sh) provides a sample post-processing script that you can use
to encrypt your backup with AES256.
