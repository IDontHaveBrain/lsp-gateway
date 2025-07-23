"""Sample Python file for LSP Gateway integration testing."""

import os
import sys
from typing import List, Optional


class TestClass:
    """Test class for LSP integration testing."""
    
    def __init__(self, field1: str, field2: int):
        self.field1 = field1
        self.field2 = field2
    
    def method(self) -> str:
        """Test method for hover and definition testing."""
        return f"Field1: {self.field1}, Field2: {self.field2}"


def test_function(input_data: str) -> str:
    """Test function for LSP integration testing."""
    return f"Processed: {input_data}"


def main():
    """Main function for testing."""
    test_obj = TestClass("Hello", 42)
    result = test_function("World")
    
    print(result)
    print(test_obj.method())
    
    if len(sys.argv) > 1:
        print(f"Argument: {sys.argv[1]}")


if __name__ == "__main__":
    main()
