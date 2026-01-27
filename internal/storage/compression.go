package storage

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

type CompressedBackend struct {
	backend     Backend
	compression string
}

func NewCompressedBackend(backend Backend, compression string) *CompressedBackend {
	return &CompressedBackend{
		backend:     backend,
		compression: compression,
	}
}

func (c *CompressedBackend) Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	if c.compression == "" {
		return c.backend.Put(ctx, bucket, key, r, size)
	}

	pr, pw := io.Pipe()

	go func() {
		var err error
		switch c.compression {
		case "gzip":
			err = c.compressGzip(pw, r)
		case "zstd":
			err = c.compressZstd(pw, r)
		default:
			err = fmt.Errorf("unsupported compression type: %s", c.compression)
		}
		pw.CloseWithError(err)
	}()

	return c.backend.Put(ctx, bucket, key, pr, -1)
}

func (c *CompressedBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	rc, err := c.backend.Get(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	if c.compression == "" {
		return rc, nil
	}

	pr, pw := io.Pipe()

	go func() {
		var err error
		switch c.compression {
		case "gzip":
			err = c.decompressGzip(pw, rc)
		case "zstd":
			err = c.decompressZstd(pw, rc)
		default:
			err = fmt.Errorf("unsupported compression type: %s", c.compression)
		}
		rc.Close()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

func (c *CompressedBackend) Delete(ctx context.Context, bucket, key string) error {
	return c.backend.Delete(ctx, bucket, key)
}

func (c *CompressedBackend) Exists(ctx context.Context, bucket, key string) (bool, error) {
	return c.backend.Exists(ctx, bucket, key)
}

func (c *CompressedBackend) compressGzip(w io.Writer, r io.Reader) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	_, err := io.Copy(gw, r)
	return err
}

func (c *CompressedBackend) decompressGzip(w io.Writer, r io.Reader) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	_, err = io.Copy(w, gr)
	return err
}

func (c *CompressedBackend) compressZstd(w io.Writer, r io.Reader) error {
	zw, err := zstd.NewWriter(w)
	if err != nil {
		return err
	}
	defer zw.Close()

	_, err = io.Copy(zw, r)
	return err
}

func (c *CompressedBackend) decompressZstd(w io.Writer, r io.Reader) error {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer zr.Close()

	_, err = io.Copy(w, zr)
	return err
}
