/**
 * A simple utility class for mathematical operations
 */
class Calculator {
  constructor() {
    this.history = [];
  }

  /**
   * Add two numbers
   * @param {number} a - First number
   * @param {number} b - Second number
   * @returns {number} The sum of a and b
   */
  add(a, b) {
    const result = a + b;
    this.history.push(`${a} + ${b} = ${result}`);
    return result;
  }

  /**
   * Multiply two numbers
   * @param {number} a - First number  
   * @param {number} b - Second number
   * @returns {number} The product of a and b
   */
  multiply(a, b) {
    const result = a * b;
    this.history.push(`${a} * ${b} = ${result}`);
    return result;
  }

  /**
   * Get calculation history
   * @returns {string[]} Array of calculation history
   */
  getHistory() {
    return this.history;
  }

  /**
   * Clear calculation history
   */
  clearHistory() {
    this.history = [];
  }
}

const calc = new Calculator();
const sum = calc.add(5, 3);
const product = calc.multiply(4, 7);
const history = calc.getHistory();

console.log(`Sum: ${sum}`);
console.log(`Product: ${product}`);
console.log('History:', history);

module.exports = Calculator;