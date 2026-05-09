package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/garyburd/redigo/redis"
	"github.com/opencontainers/go-digest"
)

func TestRepositoryScopedClearRemovesMembership(t *testing.T) {
	if redisAddr == "" {
		redisAddr = os.Getenv("TEST_REGISTRY_STORAGE_CACHE_REDIS_ADDR")
	}
	if redisAddr == "" {
		t.Skip("please set -test.registry.storage.cache.redis.addr to test layer info cache against redis")
	}

	pool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisAddr)
		},
		MaxIdle:   1,
		MaxActive: 2,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		Wait: false,
	}

	conn := pool.Get()
	if _, err := conn.Do("FLUSHDB"); err != nil {
		t.Fatalf("unexpected error flushing redis db: %v", err)
	}
	conn.Close()

	ctx := context.Background()
	provider := NewRedisBlobDescriptorCacheProvider(pool)
	cache, err := provider.RepositoryScoped("foo/bar")
	if err != nil {
		t.Fatalf("unexpected error getting scoped cache: %v", err)
	}

	localDigest := digest.Digest("sha384:def111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111")
	expected := distribution.Descriptor{
		Digest:    "sha256:def1111111111111111111111111111111111111111111111111111111111111",
		Size:      10,
		MediaType: "application/octet-stream",
	}

	if err := cache.SetDescriptor(ctx, localDigest, expected); err != nil {
		t.Fatalf("error setting descriptor: %v", err)
	}
	if err := cache.Clear(ctx, localDigest); err != nil {
		t.Fatalf("error clearing descriptor: %v", err)
	}
	if _, err := cache.Stat(ctx, localDigest); err != distribution.ErrBlobUnknown {
		t.Fatalf("expected repository-scoped blob to be cleared, got: %v", err)
	}
	if _, err := provider.Stat(ctx, localDigest); err != distribution.ErrBlobUnknown {
		t.Fatalf("expected global descriptor fields to be cleared, got: %v", err)
	}
}
