package main

import (
    "fmt"
    "os"
)

// TestStruct demonstrates a struct for testing
type TestStruct struct {
    Field1 string
    Field2 int
}

// TestFunction demonstrates a function for testing
func TestFunction(input string) string {
    return fmt.Sprintf("Processed: %s", input)
}

// Method demonstrates a method for testing
func (ts *TestStruct) Method() string {
    return ts.Field1
}

func main() {
    test := &TestStruct{
        Field1: "Hello",
        Field2: 42,
    }
    
    result := TestFunction("World")
    fmt.Println(result)
    fmt.Println(test.Method())
    
    if len(os.Args) > 1 {
        fmt.Printf("Argument: %s\n", os.Args[1])
    }
}
