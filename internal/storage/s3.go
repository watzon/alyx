package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	alyxconfig "github.com/watzon/alyx/internal/config"
)

const (
	multipartThreshold = 5 * 1024 * 1024
	partSize           = 5 * 1024 * 1024
)

type S3Backend struct {
	client       *s3.Client
	bucketPrefix string
}

func NewS3Backend(ctx context.Context, cfg alyxconfig.S3Config) (Backend, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("%w: region is required", ErrInvalidConfig)
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("%w: access_key_id is required", ErrInvalidConfig)
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("%w: secret_access_key is required", ErrInvalidConfig)
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	clientOpts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
		},
	}

	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)

	return &S3Backend{
		client:       client,
		bucketPrefix: cfg.BucketPrefix,
	}, nil
}

func (b *S3Backend) bucketName(bucket string) string {
	if b.bucketPrefix == "" {
		return bucket
	}
	return b.bucketPrefix + bucket
}

func (b *S3Backend) Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	bucketName := b.bucketName(bucket)

	if size >= multipartThreshold && size > 0 {
		return b.putMultipart(ctx, bucketName, key, r, size)
	}

	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   r,
	})
	if err != nil {
		return fmt.Errorf("putting object: %w", err)
	}

	return nil
}

func (b *S3Backend) putMultipart(ctx context.Context, bucket, key string, r io.Reader, _ int64) error {
	createResp, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("creating multipart upload: %w", err)
	}

	uploadID := createResp.UploadId
	var completedParts []types.CompletedPart
	partNumber := int32(1)

	buf := make([]byte, partSize)
	for {
		n, err := io.ReadFull(r, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("reading part: %w", err)
		}

		if n == 0 {
			break
		}

		uploadResp, err := b.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(key),
			UploadId:   uploadID,
			PartNumber: aws.Int32(partNumber),
			Body:       &readerAt{data: buf[:n]},
		})
		if err != nil {
			b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("uploading part %d: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadResp.ETag,
			PartNumber: aws.Int32(partNumber),
		})

		partNumber++

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
	}

	_, err = b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return fmt.Errorf("completing multipart upload: %w", err)
	}

	return nil
}

func (b *S3Backend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	bucketName := b.bucketName(bucket)

	resp, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting object: %w", err)
	}

	return resp.Body, nil
}

func (b *S3Backend) Delete(ctx context.Context, bucket, key string) error {
	bucketName := b.bucketName(bucket)

	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("deleting object: %w", err)
	}

	return nil
}

func (b *S3Backend) Exists(ctx context.Context, bucket, key string) (bool, error) {
	bucketName := b.bucketName(bucket)

	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("checking object existence: %w", err)
	}

	return true, nil
}

type readerAt struct {
	data []byte
	pos  int
}

func (r *readerAt) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
