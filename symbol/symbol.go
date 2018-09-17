package symbol

// SymbolType is a type for symbolic variables.
type SymbolType interface{}

// Symbols are the list of symbolic variables.
var Symbols []SymbolType

// RetValType is a type for return values.
type RetValType interface{}

// RetVals are the list of return values.
var RetVals []RetValType

// TestAssert is a marking function to make assertions for generated tests
func TestAssert(_ bool) {}
