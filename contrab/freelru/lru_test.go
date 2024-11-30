package freelru_test

import (
	"testing"
	"time"

	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"

	"github.com/stretchr/testify/require"
)

func TestUpdateLifetimeOnGet(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.AddWithLifetime("hello", "world", 2*time.Second)
	time.Sleep(time.Second)
	_, ok := lru.GetAndRefresh("hello")
	require.True(t, ok)
	time.Sleep(time.Second + time.Millisecond*100)
	_, ok = lru.Get("hello")
	require.True(t, ok)
}

func TestUpdateLifetimeOnGet1(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.AddWithLifetime("hello", "world", 2*time.Second)
	time.Sleep(time.Second)
	lru.Peek("hello")
	time.Sleep(time.Second + time.Millisecond*100)
	_, ok := lru.Get("hello")
	require.False(t, ok)
}

func TestUpdateLifetime(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.Add("hello", "world")
	require.True(t, lru.UpdateLifetime("hello", "world", 2*time.Second))
	time.Sleep(time.Second)
	_, ok := lru.Get("hello")
	require.True(t, ok)
	time.Sleep(time.Second + time.Millisecond*100)
	_, ok = lru.Get("hello")
	require.False(t, ok)
}

func TestUpdateLifetime1(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.Add("hello", "world")
	require.False(t, lru.UpdateLifetime("hello", "not world", 2*time.Second))
	time.Sleep(2*time.Second + time.Millisecond*100)
	_, ok := lru.Get("hello")
	require.True(t, ok)
}

func TestUpdateLifetime2(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.AddWithLifetime("hello", "world", 2*time.Second)
	time.Sleep(time.Second)
	require.True(t, lru.UpdateLifetime("hello", "world", 2*time.Second))
	time.Sleep(time.Second + time.Millisecond*100)
	_, ok := lru.Get("hello")
	require.True(t, ok)
	time.Sleep(time.Second + time.Millisecond*100)
	_, ok = lru.Get("hello")
	require.False(t, ok)
}

func TestPeekWithLifetime(t *testing.T) {
	t.Parallel()
	lru, err := freelru.New[string, string](1024, maphash.NewHasher[string]().Hash32)
	require.NoError(t, err)
	lru.SetLifetime(time.Second)
	lru.AddWithLifetime("hello", "world", 10*time.Second)
	lru.Add("hello1", "")
	lru.Add("hello2", "")
	lru.Add("hello3", "")
	time.Sleep(2 * time.Second)
	lru.PurgeExpired()
	require.Equal(t, 1, lru.Len())
}
