package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	ContentTypeM4SSegment = "audio/mp4"
	ContentTypeInitMP4    = "audio/mp4"
	ContentTypeMPD        = "application/dash+xml"
)

type ObjectClient interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type Service struct {
	bucket string
	client ObjectClient
}

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	ContentType string
	SizeBytes   *int64
	Metadata    map[string]string
}

type GetObjectInput struct {
	Key   string
	Range string
}

type ListObjectsInput struct {
	Prefix string
	Cursor string
	Limit  int32
}

type ObjectInfo struct {
	Bucket       string
	Key          string
	ETag         string
	ContentType  string
	SizeBytes    *int64
	LastModified *time.Time
	Metadata     map[string]string
}

type GetObjectResult struct {
	Info ObjectInfo
	Body io.ReadCloser
}

type ListObjectsResult struct {
	Objects    []ObjectInfo
	NextCursor *string
}

func NewService(bucket string, client ObjectClient) (*Service, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}
	if client == nil {
		return nil, fmt.Errorf("storage client is required")
	}

	return &Service{
		bucket: bucket,
		client: client,
	}, nil
}

func (s *Service) Bucket() string {
	if s == nil {
		return ""
	}

	return s.bucket
}

func (s *Service) HeadBucket(ctx context.Context) error {
	if err := s.ensureConfigured(); err != nil {
		return err
	}

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("head storage bucket: %w", err)
	}

	return nil
}

func (s *Service) PutObject(ctx context.Context, input PutObjectInput) (ObjectInfo, error) {
	if err := s.ensureConfigured(); err != nil {
		return ObjectInfo{}, err
	}

	key, err := objectKey(input.Key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if input.Body == nil {
		return ObjectInfo{}, fmt.Errorf("storage object body is required")
	}
	if input.SizeBytes != nil && *input.SizeBytes < 0 {
		return ObjectInfo{}, fmt.Errorf("storage object sizeBytes must be greater than or equal to 0")
	}

	putInput := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          input.Body,
		ContentLength: input.SizeBytes,
		Metadata:      copyMetadata(input.Metadata),
	}
	if contentType := strings.TrimSpace(input.ContentType); contentType != "" {
		putInput.ContentType = aws.String(contentType)
	}

	output, err := s.client.PutObject(ctx, putInput)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("put storage object %q: %w", key, err)
	}

	return ObjectInfo{
		Bucket:      s.bucket,
		Key:         key,
		ETag:        aws.ToString(output.ETag),
		ContentType: strings.TrimSpace(input.ContentType),
		SizeBytes:   input.SizeBytes,
		Metadata:    copyMetadata(input.Metadata),
	}, nil
}

func (s *Service) GetObject(ctx context.Context, input GetObjectInput) (GetObjectResult, error) {
	if err := s.ensureConfigured(); err != nil {
		return GetObjectResult{}, err
	}

	key, err := objectKey(input.Key)
	if err != nil {
		return GetObjectResult{}, err
	}

	getInput := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if byteRange := strings.TrimSpace(input.Range); byteRange != "" {
		getInput.Range = aws.String(byteRange)
	}

	output, err := s.client.GetObject(ctx, getInput)
	if err != nil {
		return GetObjectResult{}, fmt.Errorf("get storage object %q: %w", key, err)
	}

	return GetObjectResult{
		Info: ObjectInfo{
			Bucket:       s.bucket,
			Key:          key,
			ETag:         aws.ToString(output.ETag),
			ContentType:  aws.ToString(output.ContentType),
			SizeBytes:    output.ContentLength,
			LastModified: output.LastModified,
			Metadata:     copyMetadata(output.Metadata),
		},
		Body: output.Body,
	}, nil
}

func (s *Service) HeadObject(ctx context.Context, key string) (ObjectInfo, error) {
	if err := s.ensureConfigured(); err != nil {
		return ObjectInfo{}, err
	}

	key, err := objectKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}

	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("head storage object %q: %w", key, err)
	}

	return ObjectInfo{
		Bucket:       s.bucket,
		Key:          key,
		ETag:         aws.ToString(output.ETag),
		ContentType:  aws.ToString(output.ContentType),
		SizeBytes:    output.ContentLength,
		LastModified: output.LastModified,
		Metadata:     copyMetadata(output.Metadata),
	}, nil
}

func (s *Service) DeleteObject(ctx context.Context, key string) error {
	if err := s.ensureConfigured(); err != nil {
		return err
	}

	key, err := objectKey(key)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete storage object %q: %w", key, err)
	}

	return nil
}

func (s *Service) ListObjects(ctx context.Context, input ListObjectsInput) (ListObjectsResult, error) {
	if err := s.ensureConfigured(); err != nil {
		return ListObjectsResult{}, err
	}
	if input.Limit < 0 {
		return ListObjectsResult{}, fmt.Errorf("storage list limit must be greater than or equal to 0")
	}

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}
	if prefix := strings.TrimSpace(input.Prefix); prefix != "" {
		listInput.Prefix = aws.String(prefix)
	}
	if cursor := strings.TrimSpace(input.Cursor); cursor != "" {
		listInput.ContinuationToken = aws.String(cursor)
	}
	if input.Limit > 0 {
		listInput.MaxKeys = aws.Int32(input.Limit)
	}

	output, err := s.client.ListObjectsV2(ctx, listInput)
	if err != nil {
		return ListObjectsResult{}, fmt.Errorf("list storage objects: %w", err)
	}

	objects := make([]ObjectInfo, 0, len(output.Contents))
	for _, object := range output.Contents {
		objects = append(objects, objectInfoFromListObject(s.bucket, object))
	}

	return ListObjectsResult{
		Objects:    objects,
		NextCursor: output.NextContinuationToken,
	}, nil
}

func (s *Service) ensureConfigured() error {
	if s == nil || s.bucket == "" || s.client == nil {
		return fmt.Errorf("storage service is not configured")
	}

	return nil
}

func objectKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("storage object key is required")
	}

	return key, nil
}

func objectInfoFromListObject(bucket string, object types.Object) ObjectInfo {
	return ObjectInfo{
		Bucket:       bucket,
		Key:          aws.ToString(object.Key),
		ETag:         aws.ToString(object.ETag),
		SizeBytes:    object.Size,
		LastModified: object.LastModified,
	}
}

func copyMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	copied := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}

	return copied
}
