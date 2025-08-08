package jsonutil

import "encoding/json"

func Convert[T any](v any) (T, error) {
    var out T
    b, err := json.Marshal(v)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

