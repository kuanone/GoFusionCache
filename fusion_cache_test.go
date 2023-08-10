package fusion_cache

import (
	"context"
	gocache "github.com/patrickmn/go-cache"
	"testing"
	"time"
)

func TestNewDefaultFusionCache(t *testing.T) {
	type args struct {
		dsn               string
		defaultExpiration time.Duration
		cleanupInterval   time.Duration
	}
	tests := []struct {
		name string
		args args
		want *FusionCache[string, string]
	}{
		{
			name: "test",
			args: args{
				dsn:               "redis://:@127.0.0.1:6379/0",
				defaultExpiration: gocache.DefaultExpiration,
				cleanupInterval:   gocache.DefaultExpiration,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDefaultFusionCache(tt.args.dsn, tt.args.defaultExpiration, tt.args.cleanupInterval)
			t.Log(got.Set(context.Background(), "test", "test"))
			t.Log(got.Get(context.Background(), "test"))
		})
	}
}
