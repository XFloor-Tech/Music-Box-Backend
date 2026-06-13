package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
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

type PresignClient interface {
	PresignPutObject(context.Context, *s3.PutObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type PresignConfig struct {
	PutExpiry time.Duration
	GetExpiry time.Duration
}

type Service struct {
	bucket        string
	client        ObjectClient
	presigner     PresignClient
	presignConfig PresignConfig
}

type PresignPutObjectInput struct {
	Folder      string
	FileName    string
	ContentType string
	SizeBytes   *int64
	Metadata    map[string]string
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

type PresignedURL struct {
	Bucket    string
	Key       string
	URL       string
	Method    string
	Headers   http.Header
	ExpiresIn time.Duration
	ExpiresAt time.Time
}

func NewService(bucket string, client ObjectClient, presigner PresignClient, presignConfig PresignConfig) (*Service, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}
	if client == nil {
		return nil, fmt.Errorf("storage client is required")
	}
	if presigner == nil {
		return nil, fmt.Errorf("storage presigner is required")
	}
	if err := validatePresignConfig(presignConfig); err != nil {
		return nil, err
	}

	return &Service{
		bucket:        bucket,
		client:        client,
		presigner:     presigner,
		presignConfig: presignConfig,
	}, nil
}

func (s *Service) Bucket() string {
	if s == nil {
		return ""
	}

	return s.bucket
}

func (s *Service) PresignPutObject(ctx context.Context, input PresignPutObjectInput) (PresignedURL, error) {
	if err := s.ensurePresignerConfigured(); err != nil {
		return PresignedURL{}, err
	}

	key, err := ObjectKey(input.Folder, input.FileName)
	if err != nil {
		return PresignedURL{}, err
	}
	if input.SizeBytes != nil && *input.SizeBytes < 0 {
		return PresignedURL{}, fmt.Errorf("storage object sizeBytes must be greater than or equal to 0")
	}

	putInput := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentLength: input.SizeBytes,
		Metadata:      copyMetadata(input.Metadata),
	}
	if contentType := strings.TrimSpace(input.ContentType); contentType != "" {
		putInput.ContentType = aws.String(contentType)
	}

	expiry := s.presignConfig.PutExpiry
	request, err := s.presigner.PresignPutObject(ctx, putInput, s3.WithPresignExpires(expiry))
	if err != nil {
		return PresignedURL{}, fmt.Errorf("presign put storage object %q: %w", key, err)
	}

	return presignedURLFromRequest(s.bucket, key, request, expiry), nil
}

func (s *Service) PresignGetObject(ctx context.Context, key string) (PresignedURL, error) {
	if err := s.ensurePresignerConfigured(); err != nil {
		return PresignedURL{}, err
	}

	key, err := objectKey(key)
	if err != nil {
		return PresignedURL{}, err
	}

	expiry := s.presignConfig.GetExpiry
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return PresignedURL{}, fmt.Errorf("presign get storage object %q: %w", key, err)
	}

	return presignedURLFromRequest(s.bucket, key, request, expiry), nil
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

func (s *Service) ensurePresignerConfigured() error {
	if s == nil || s.bucket == "" || s.presigner == nil {
		return fmt.Errorf("storage presigner is not configured")
	}
	if err := validatePresignConfig(s.presignConfig); err != nil {
		return err
	}

	return nil
}

func validatePresignConfig(cfg PresignConfig) error {
	if cfg.PutExpiry <= 0 {
		return fmt.Errorf("storage presign put expiry must be greater than 0")
	}
	if cfg.PutExpiry > maxPresignExpiry {
		return fmt.Errorf("storage presign put expiry must be less than or equal to %s", maxPresignExpiry)
	}
	if cfg.GetExpiry <= 0 {
		return fmt.Errorf("storage presign get expiry must be greater than 0")
	}
	if cfg.GetExpiry > maxPresignExpiry {
		return fmt.Errorf("storage presign get expiry must be less than or equal to %s", maxPresignExpiry)
	}

	return nil
}

func ObjectKey(folder string, fileName string) (string, error) {
	folder, err := normalizeObjectFolder(folder)
	if err != nil {
		return "", err
	}

	fileName, err = normalizeObjectFileName(fileName)
	if err != nil {
		return "", err
	}

	if folder == "" {
		return objectKey(fileName)
	}

	return objectKey(folder + "/" + fileName)
}

func objectKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("storage object key is required")
	}

	return key, nil
}

func normalizeObjectFolder(folder string) (string, error) {
	folder = strings.Trim(strings.TrimSpace(folder), "/")
	if folder == "" {
		return "", nil
	}

	parts := strings.Split(folder, "/")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			return "", fmt.Errorf("storage object folder must not contain relative path segments")
		}

		normalized = append(normalized, part)
	}
	if len(normalized) == 0 {
		return "", nil
	}

	return strings.Join(normalized, "/"), nil
}

func normalizeObjectFileName(fileName string) (string, error) {
	fileName = strings.Trim(strings.TrimSpace(fileName), "/")
	if fileName == "" {
		return "", fmt.Errorf("storage object fileName is required")
	}
	if strings.Contains(fileName, "/") {
		return "", fmt.Errorf("storage object fileName must not contain path separators")
	}
	if fileName == "." || fileName == ".." {
		return "", fmt.Errorf("storage object fileName must not be a relative path segment")
	}

	return fileName, nil
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

func presignedURLFromRequest(bucket string, key string, request *v4.PresignedHTTPRequest, expiry time.Duration) PresignedURL {
	headers := http.Header{}
	if request != nil && request.SignedHeader != nil {
		headers = request.SignedHeader.Clone()
	}

	result := PresignedURL{
		Bucket:    bucket,
		Key:       key,
		Headers:   headers,
		ExpiresIn: expiry,
		ExpiresAt: time.Now().UTC().Add(expiry),
	}
	if request != nil {
		result.URL = request.URL
		result.Method = request.Method
	}

	return result
}
