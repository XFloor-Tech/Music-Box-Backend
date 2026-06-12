package storage

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestPutObjectUsesConfiguredBucketAndObjectMetadata(t *testing.T) {
	client := &recordingObjectClient{
		putOutput: &s3.PutObjectOutput{
			ETag: aws.String(`"etag123"`),
		},
	}
	service := testService(t, client)
	sizeBytes := int64(42)

	info, err := service.PutObject(context.Background(), PutObjectInput{
		Key:         " tracks/trk_123/init.mp4 ",
		Body:        strings.NewReader("init"),
		ContentType: ContentTypeInitMP4,
		SizeBytes:   &sizeBytes,
		Metadata: map[string]string{
			"trackId": "trk_123",
		},
	})
	if err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}

	if aws.ToString(client.putInput.Bucket) != "music-box-media" {
		t.Fatalf("put bucket = %q, want configured bucket", aws.ToString(client.putInput.Bucket))
	}
	if aws.ToString(client.putInput.Key) != "tracks/trk_123/init.mp4" {
		t.Fatalf("put key = %q, want trimmed key", aws.ToString(client.putInput.Key))
	}
	if aws.ToString(client.putInput.ContentType) != ContentTypeInitMP4 {
		t.Fatalf("put content type = %q, want %q", aws.ToString(client.putInput.ContentType), ContentTypeInitMP4)
	}
	if client.putInput.ContentLength == nil || *client.putInput.ContentLength != sizeBytes {
		t.Fatalf("put content length = %v, want %d", client.putInput.ContentLength, sizeBytes)
	}
	if client.putInput.Metadata["trackId"] != "trk_123" {
		t.Fatalf("put metadata = %#v, want trackId", client.putInput.Metadata)
	}
	if info.ETag != `"etag123"` {
		t.Fatalf("info ETag = %q, want etag123", info.ETag)
	}
}

func TestGetObjectReturnsBodyAndMetadata(t *testing.T) {
	lastModified := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	sizeBytes := int64(12)
	client := &recordingObjectClient{
		getOutput: &s3.GetObjectOutput{
			Body:          io.NopCloser(strings.NewReader("segment")),
			ContentLength: &sizeBytes,
			ContentType:   aws.String(ContentTypeM4SSegment),
			ETag:          aws.String(`"etag456"`),
			LastModified:  &lastModified,
			Metadata: map[string]string{
				"trackId": "trk_123",
			},
		},
	}
	service := testService(t, client)

	result, err := service.GetObject(context.Background(), GetObjectInput{
		Key:   "tracks/trk_123/segment-1.m4s",
		Range: "bytes=0-99",
	})
	if err != nil {
		t.Fatalf("GetObject() error = %v", err)
	}
	defer result.Body.Close()

	if aws.ToString(client.getInput.Range) != "bytes=0-99" {
		t.Fatalf("get range = %q, want bytes range", aws.ToString(client.getInput.Range))
	}
	if result.Info.ContentType != ContentTypeM4SSegment {
		t.Fatalf("content type = %q, want %q", result.Info.ContentType, ContentTypeM4SSegment)
	}
	if result.Info.SizeBytes == nil || *result.Info.SizeBytes != sizeBytes {
		t.Fatalf("sizeBytes = %v, want %d", result.Info.SizeBytes, sizeBytes)
	}
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "segment" {
		t.Fatalf("body = %q, want segment", string(body))
	}
}

func TestListObjectsUsesPrefixCursorAndLimit(t *testing.T) {
	nextCursor := "next-token"
	sizeBytes := int64(128)
	client := &recordingObjectClient{
		listOutput: &s3.ListObjectsV2Output{
			NextContinuationToken: &nextCursor,
			Contents: []types.Object{
				{
					Key:  aws.String("tracks/trk_123/segment-1.m4s"),
					ETag: aws.String(`"etag789"`),
					Size: &sizeBytes,
				},
			},
		},
	}
	service := testService(t, client)

	result, err := service.ListObjects(context.Background(), ListObjectsInput{
		Prefix: "tracks/trk_123/",
		Cursor: "cursor-token",
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}

	if aws.ToString(client.listInput.Prefix) != "tracks/trk_123/" {
		t.Fatalf("prefix = %q, want track prefix", aws.ToString(client.listInput.Prefix))
	}
	if aws.ToString(client.listInput.ContinuationToken) != "cursor-token" {
		t.Fatalf("cursor = %q, want cursor-token", aws.ToString(client.listInput.ContinuationToken))
	}
	if client.listInput.MaxKeys == nil || *client.listInput.MaxKeys != 25 {
		t.Fatalf("max keys = %v, want 25", client.listInput.MaxKeys)
	}
	if len(result.Objects) != 1 || result.Objects[0].Key != "tracks/trk_123/segment-1.m4s" {
		t.Fatalf("objects = %#v, want one listed object", result.Objects)
	}
	if result.NextCursor == nil || *result.NextCursor != nextCursor {
		t.Fatalf("next cursor = %v, want %q", result.NextCursor, nextCursor)
	}
}

func TestDeleteObjectUsesConfiguredBucket(t *testing.T) {
	client := &recordingObjectClient{}
	service := testService(t, client)

	if err := service.DeleteObject(context.Background(), "tracks/trk_123/manifest.m3u8"); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}

	if aws.ToString(client.deleteInput.Bucket) != "music-box-media" {
		t.Fatalf("delete bucket = %q, want configured bucket", aws.ToString(client.deleteInput.Bucket))
	}
	if aws.ToString(client.deleteInput.Key) != "tracks/trk_123/manifest.m3u8" {
		t.Fatalf("delete key = %q, want manifest key", aws.ToString(client.deleteInput.Key))
	}
}

func testService(t *testing.T, client ObjectClient) *Service {
	t.Helper()

	service, err := NewService("music-box-media", client)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return service
}

type recordingObjectClient struct {
	headBucketInput *s3.HeadBucketInput
	putInput        *s3.PutObjectInput
	putOutput       *s3.PutObjectOutput
	getInput        *s3.GetObjectInput
	getOutput       *s3.GetObjectOutput
	headInput       *s3.HeadObjectInput
	headOutput      *s3.HeadObjectOutput
	deleteInput     *s3.DeleteObjectInput
	listInput       *s3.ListObjectsV2Input
	listOutput      *s3.ListObjectsV2Output
}

func (c *recordingObjectClient) HeadBucket(ctx context.Context, input *s3.HeadBucketInput, options ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	c.headBucketInput = input
	return &s3.HeadBucketOutput{}, nil
}

func (c *recordingObjectClient) PutObject(ctx context.Context, input *s3.PutObjectInput, options ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.putInput = input
	if c.putOutput != nil {
		return c.putOutput, nil
	}

	return &s3.PutObjectOutput{}, nil
}

func (c *recordingObjectClient) GetObject(ctx context.Context, input *s3.GetObjectInput, options ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.getInput = input
	if c.getOutput != nil {
		return c.getOutput, nil
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("")),
	}, nil
}

func (c *recordingObjectClient) HeadObject(ctx context.Context, input *s3.HeadObjectInput, options ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	c.headInput = input
	if c.headOutput != nil {
		return c.headOutput, nil
	}

	return &s3.HeadObjectOutput{}, nil
}

func (c *recordingObjectClient) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, options ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	c.deleteInput = input
	return &s3.DeleteObjectOutput{}, nil
}

func (c *recordingObjectClient) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	c.listInput = input
	if c.listOutput != nil {
		return c.listOutput, nil
	}

	return &s3.ListObjectsV2Output{}, nil
}
