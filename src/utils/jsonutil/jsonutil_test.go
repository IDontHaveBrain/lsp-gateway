package jsonutil

import "testing"

func TestConvert_StructToMap(t *testing.T) {
    type S struct{ A int; B string }
    in := S{A: 42, B: "x"}
    out, err := Convert[map[string]interface{}](in)
    if err != nil {
        t.Fatalf("Convert error: %v", err)
    }
    if out["A"].(float64) != 42 || out["B"].(string) != "x" {
        t.Fatalf("unexpected convert result: %#v", out)
    }
}

func TestConvert_TypeMismatchReturnsError(t *testing.T) {
    _, err := Convert[int]("123")
    if err == nil {
        t.Fatalf("expected error converting string to int")
    }
}

