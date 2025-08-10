package common

import (
    "strings"
    "testing"

    "lsp-gateway/src/server/cache"
)

func TestFormatBytes(t *testing.T) {
    cases := []struct{ in int64; wantSub string }{
        {500, "500 B"},
        {1024, "1.0 KB"},
        {1536, "1.5 KB"},
        {1024 * 1024, "1.0 MB"},
    }
    for _, c := range cases {
        got := FormatBytes(c.in)
        if !strings.Contains(got, c.wantSub) {
            t.Fatalf("FormatBytes(%d)=%q, want contains %q", c.in, got, c.wantSub)
        }
    }
}

func TestFormatCacheMetrics(t *testing.T) {
    if got := FormatCacheMetrics(nil); got != "No metrics available" {
        t.Fatalf("unexpected nil metrics text: %q", got)
    }

    m := &cache.CacheMetrics{HitCount: 3, MissCount: 1, EntryCount: 2, TotalSize: 2048}
    got := FormatCacheMetrics(m)
    if !strings.Contains(got, "2 entries") || !strings.Contains(got, "(3/4 requests)") {
        t.Fatalf("unexpected metrics: %q", got)
    }
}

