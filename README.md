# ssmenv-go
[![Go Reference](https://pkg.go.dev/badge/github.com/Finatext/ssmenv-go.svg)](https://pkg.go.dev/github.com/Finatext/ssmenv-go)

ssmenv-go provides a way to replace environment variables with AWS Systems Manager Parameter Store values.

If the value of an environment variable begins with `ssm://`, it will be replaced by the corresponding SSM parameter value.
If no environment variable starts with `ssm://`, no API calls are made, and the original environment variables are returned unchanged.

In the following example, ssmenv-go fetches the value stored under the `/some_parameter/path` key, and the returned map will contain the fetched value instead of the original key.

```go
os.Setenv("SOME_ENV", "ssm:///some_parameter/path")

awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
if err != nil {
  return errors.Wrap(err, "failed to load AWS config")
}
ssmClient := ssm.NewFromConfig(awsConfig)
replacedEnv, err := ssmenv.ReplacedEnv(ctx, ssmClient, os.Environ())
```

The complete code is available at https://github.com/Finatext/belldog/blob/main/cmd/lambda/lambda.go

Use the returned `os.Environ()` compatible slice with tools like envconfig. An example with [env](https://github.com/caarlos0/env) package:

```go
replacedEnv, err := ssmenv.ReplacedEnv(ctx, ssmClient, os.Environ())
if err != nil {
  return errors.Wrap(err, "failed to fetch replaced env")
}
config, err := env.ParseAsWithOptions[appconfig.Config](env.Options{
  Environment: replacedEnv,
})
if err != nil {
  return errors.Wrap(err, "failed to process config from env")
}
```

## IAM permissions
ssmenv-go uses `ssm:GetParameters` operation.

## Acknowledgements
The approach of replacing environment variable values that start with the `ssm://` format was inspired by [remind101/ssm-env](https://github.com/remind101/ssm-env).

The approach used by ssmenv-go (this library) differs from ssm-env in that it avoids passing secrets through environment variables, as this is generally not considered a best practice for security. Environment variables can sometimes be exposed in logs, error messages, or system dumps, so it's safer to handle sensitive data directly within the application.
