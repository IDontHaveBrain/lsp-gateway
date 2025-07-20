/**
 * Sample TypeScript file for LSP Gateway integration testing
 */

interface TestInterface {
    field1: string;
    field2: number;
}

class TestClass implements TestInterface {
    field1: string;
    field2: number;
    
    constructor(field1: string, field2: number) {
        this.field1 = field1;
        this.field2 = field2;
    }
    
    method(): string {
        return `Field1: ${this.field1}, Field2: ${this.field2}`;
    }
}

function testFunction(input: string): string {
    return `Processed: ${input}`;
}

function main(): void {
    const testObj = new TestClass("Hello", 42);
    const result = testFunction("World");
    
    console.log(result);
    console.log(testObj.method());
    
    if (process.argv.length > 2) {
        console.log(`Argument: ${process.argv[2]}`);
    }
}

if (require.main === module) {
    main();
}

export { TestClass, TestInterface, testFunction };
