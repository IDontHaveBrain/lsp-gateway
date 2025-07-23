/**
 * Sample JavaScript file for LSP Gateway integration testing
 */

class TestClass {
    constructor(field1, field2) {
        this.field1 = field1;
        this.field2 = field2;
    }
    
    method() {
        return `Field1: ${this.field1}, Field2: ${this.field2}`;
    }
}

function testFunction(input) {
    return `Processed: ${input}`;
}

function main() {
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

module.exports = { TestClass, testFunction };
