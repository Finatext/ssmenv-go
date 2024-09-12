// ssmenv provides a way to replace environment variables with AWS Systems Manager Parameter Store values.
// If an environment variable value starts with "ssm://", it will be replaced with the value of the SSM parameter.
// If no environment variable starts with "ssm://", the original environment variables are returned.
package ssmenv

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// InvalidEnvVarFormatError is returned when given environment variables have an invalid format.
type InvalidEnvVarFormatError struct {
	OriginalEnvVar string
}

func (e InvalidEnvVarFormatError) Error() string {
	return fmt.Sprintf("invalid environment variable format: %s", e.OriginalEnvVar)
}

// ParameterNotFoundError is returned when the SSM parameter is not found.
type ParameterNotFoundError struct {
	Key string
}

func (e ParameterNotFoundError) Error() string {
	return fmt.Sprintf("parameter not found: %s", e.Key)
}

// GetParametersError is returned when GetParameters operation fails.
type GetParametersError struct {
	// Cause contains the original error which AWS SDK returned.
	Cause error
}

func (e GetParametersError) Error() string {
	return fmt.Sprintf("failed to get SSM parameters: %v", e.Cause)
}

func (e GetParametersError) Unwrap() error {
	return e.Cause
}

// InvalidParametersError is returned when AWS API returns invalid parameters response.
type InvalidParametersError struct {
	InvalidParameters []string
}

func (e InvalidParametersError) Error() string {
	return fmt.Sprintf("invalid SSM parameters respond: %v", e.InvalidParameters)
}

// NullParameterError is returned when AWS API returns a parameter with null name or value.
type NullParameterError struct {
}

func (e NullParameterError) Error() string {
	return "null parameter parameter response"
}

// ReplacedEnv replaces environment variable values with corresponding SSM parameter values. If the value of an
// environment variable begins with "ssm://", it is replaced by the corresponding SSM parameter value.
//
// `cli` is the AWS SSM client used to retrieve the parameters. `envs` is a list of environment variables in the format
// "KEY=VALUE", similar to what is returned by os.Environ().
//
// If no environment variable starts with "ssm://", no API calls are made, and the original environment variables are
// returned unchanged.
//
// ReplacedEnv returns a map of environment variables, where values are replaced with SSM parameter values as needed.
//
// This function may return an error. Refer to the package's error definitions for details.
func ReplacedEnv(ctx context.Context, cli ssmClient, envs []string) (map[string]string, error) {
	orig := make(map[string]string)
	ssmKeys := []string{}

	for _, env := range envs {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			return nil, InvalidEnvVarFormatError{OriginalEnvVar: env}
		}
		key := pair[0]
		value := pair[1]
		orig[key] = value

		if strings.HasPrefix(value, ssmPrefix) {
			ssmKeys = append(ssmKeys, strings.TrimPrefix(value, ssmPrefix))
		}
	}

	if len(ssmKeys) == 0 {
		return orig, nil
	}

	slog.InfoContext(ctx, "fetching SSM parameters", slog.String("keys", strings.Join(ssmKeys, ",")))
	ps, err := batchFetch(ctx, cli, ssmKeys)
	if err != nil {
		return nil, err
	}
	for k, v := range orig {
		if strings.HasPrefix(v, ssmPrefix) {
			// Remove prefix, use strings.TrimPrefix
			key := strings.TrimPrefix(v, ssmPrefix)
			val, ok := ps[key]
			if !ok {
				return nil, ParameterNotFoundError{Key: key}
			}

			orig[k] = val
		}
	}

	return orig, nil
}

const ssmPrefix = "ssm://"

type ssmClient interface {
	GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error)
}

func batchFetch(ctx context.Context, cli ssmClient, keys []string) (map[string]string, error) {
	input := ssm.GetParametersInput{
		Names:          keys,
		WithDecryption: aws.Bool(true),
	}
	res, err := cli.GetParameters(ctx, &input)
	if err != nil {
		return nil, GetParametersError{Cause: err}
	}
	if len(res.InvalidParameters) > 0 {
		return nil, InvalidParametersError{res.InvalidParameters}
	}

	ret := make(map[string]string)
	for _, p := range res.Parameters {
		if p.Name == nil || p.Value == nil {
			return nil, NullParameterError{}
		}
		ret[*p.Name] = *p.Value
	}
	return ret, nil
}
