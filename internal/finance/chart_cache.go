package finance

import (
	"sync"
	"time"
)

var (
	chartCache   = map[string]chartCacheEntry{}
	chartCacheMu sync.Mutex
)

func cacheGet(key string) ([]byte, bool) {
	chartCacheMu.Lock()
	defer chartCacheMu.Unlock()
	if entry, ok := chartCache[key]; ok {
		if time.Now().Before(entry.createdAt.Add(chartCacheTTL)) {
			img := make([]byte, len(entry.image))
			copy(img, entry.image)
			return img, true
		}
	}
	return nil, false
}

func cacheSet(key string, img []byte) {
	chartCacheMu.Lock()
	chartCache[key] = chartCacheEntry{createdAt: time.Now(), image: img}
	chartCacheMu.Unlock()
}
