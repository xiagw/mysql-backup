* mysql backup itself
* unit tests
* integration tests
* parsing non-transferred cmdline options


Non-transferred cmdline options:

* `AWS_CLI_OPTS`: Additional arguments to be passed to the `aws` part of the `aws s3 cp` command, click [here](https://docs.aws.amazon.com/cli/latest/reference/#options) for a list. _Be careful_, as you can break something!
* `AWS_CLI_S3_CP_OPTS`: Additional arguments to be passed to the `s3 cp` part of the `aws s3 cp` command, click [here](https://docs.aws.amazon.com/cli/latest/reference/s3/cp.html#options) for a list. If you are using AWS KMS, `sse`, `sse-kms-key-id`, etc., may be of interest.
* `MYSQLDUMP_OPTS`: A string of options to pass to `mysqldump`, e.g. `MYSQLDUMP_OPTS="--opt abc --param def --max_allowed_packet=123455678"` will run `mysqldump --opt abc --param def --max_allowed_packet=123455678`
