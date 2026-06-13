package storage

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewR2Client(ctx context.Context, cfg Config) (*s3.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	awsConfig, err := awscfg.LoadDefaultConfig(
		ctx,
		awscfg.WithRegion(r2Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	endpoint := cfg.EndpointURL()
	return s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
	}), nil
}

func NewR2PresignClient(client *s3.Client) *s3.PresignClient {
	return s3.NewPresignClient(client)
}
