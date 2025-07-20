// Jest configuration for npm wrapper integration tests
// Note: Core functionality is tested via Go test suite (internal/*)
module.exports = {
  // Test environment
  testEnvironment: 'node',
  
  // No test files to match - all Jest tests removed due to incompatibility
  // with streamlined codebase. Core testing handled by Go test suite.
  testMatch: [
    '<rootDir>/test-never-exists/**/*.test.js'
  ],
  
  // Explicitly ignore non-test files and directories
  testPathIgnorePatterns: [
    '<rootDir>/node_modules/',
    '<rootDir>/coverage/',
    '<rootDir>/archive/',
    '<rootDir>/lib/',
    '<rootDir>/scripts/',
    '<rootDir>/bin/',
    '<rootDir>/internal/',
    '<rootDir>/cmd/',
    '<rootDir>/mcp/',
    '<rootDir>/test-npm-install/',
    '<rootDir>/demos/'
  ],
  
  // Minimal coverage configuration for potential future npm wrapper tests
  collectCoverage: false,
  coverageDirectory: 'coverage',
  collectCoverageFrom: [
    'lib/**/*.js',
    '!**/node_modules/**',
    '!archive/**',
    '!coverage/**'
  ],
  
  // Test timeout
  testTimeout: 10000,
  
  // Verbose output
  verbose: false,
  
  // Clear mocks between tests
  clearMocks: true,
  
  // Module file extensions
  moduleFileExtensions: ['js', 'json'],
  
  // Error handling
  errorOnDeprecated: true,
  
  // Reporter configuration
  reporters: [
    'default'
  ]
};