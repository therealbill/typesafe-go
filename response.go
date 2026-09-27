package typesafe

// Usage is defined fully in Task 4.
type Usage struct{ InputTokens, OutputTokens *int }

// SystemOneResponse is defined fully in Task 4.
type SystemOneResponse struct{}

// Question and Questions are defined fully in Task 2 in question.go. These
// placeholders match the final declarations so that instrument.go compiles
// now; question.go replaces them.
type Question interface{ question() }

// Questions maps question identifiers to questions.
type Questions map[string]Question
