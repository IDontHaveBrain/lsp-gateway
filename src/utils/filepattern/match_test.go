package filepattern

import (
    "path/filepath"
    "testing"
)

func TestMatch_SimplePatterns(t *testing.T) {
    if !Match("/test/file.txt", "*.txt") {
        t.Fatalf("*.txt should match")
    }
    if !Match("/src/server/main.go", "src/**/*.go") {
        t.Fatalf("recursive glob should match")
    }
    if !Match("/src/server/main.go", "server/*.go") {
        t.Fatalf("dir pattern should match")
    }
    if !Match("/a/b/c.txt", "c.txt") {
        t.Fatalf("basename match should work")
    }
}

func TestMatch_RelativeAbsolute(t *testing.T) {
    wd, _ := filepath.Abs(".")
    p := filepath.Join(wd, "src", "pkg", "x.go")
    if !Match(p, "src/**/*.go") {
        t.Fatalf("absolute should match relative pattern")
    }
}

