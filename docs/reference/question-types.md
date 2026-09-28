---
title: "Question Types"
description: "The Question interface and its implementations (Noul, Choice, Score, RawQuestion), JSONContent, and the wire JSON each question type produces."
diataxis: reference
weight: 20
---

Package: `github.com/therealbill/typesafe-go` (import path `typesafe`).

## JSONContent

```go
type JSONContent = any
```

JSONContent is text, a JSON object, or a JSON array. Valid values are a
string, a map with string keys, a slice or array, `json.RawMessage`, or a
struct (or pointer to one) that encodes to a JSON object.

## Question

```go
type Question interface {
    // Has unexported methods.
}
```

Question is one of `Noul`, `Choice`, `Score`, or `RawQuestion`. Pointers to
the struct types (`*Noul`, `*Choice`, `*Score`) are accepted too.

## Questions

```go
type Questions map[string]Question
```

Questions maps question identifiers to questions. Identifiers name the
answers in the response; they are not shown to the model.

## NoulCriteria

```go
type NoulCriteria struct {
    True  JSONContent
    False JSONContent
}
```

NoulCriteria optionally describes the yes and no outcomes of a Noul.

## Noul

```go
type Noul struct {
    Instructions JSONContent
    Criteria     *NoulCriteria
}
```

Noul asks whether a condition holds. The answer is the probability of yes.

### (Noul) MarshalJSON

```go
func (q Noul) MarshalJSON() ([]byte, error)
```

MarshalJSON encodes the question in the API's wire format.

Wire shape:

```json
{
  "type": "noul",
  "instructions": <JSONContent>,
  "criteria": {
    "true": <JSONContent>,
    "false": <JSONContent>
  }
}
```

- `"type"` is always present with the value `"noul"`.
- `"instructions"` is present only when `Instructions` is non-nil.
- `"criteria"` is present only when `Criteria` is a non-nil `*NoulCriteria`.
- Within `"criteria"`, `"true"` is present only when `Criteria.True` is
  non-nil, and `"false"` is present only when `Criteria.False` is non-nil.

## Choice

```go
type Choice struct {
    Instructions JSONContent
    Criteria     map[string]JSONContent
}
```

Choice asks for one label out of a defined set. Criteria maps each label to
a description, or nil to leave it undescribed. Between 1 and 255 labels.

### (Choice) MarshalJSON

```go
func (q Choice) MarshalJSON() ([]byte, error)
```

MarshalJSON encodes the question in the API's wire format.

Wire shape:

```json
{
  "type": "choice",
  "instructions": <JSONContent>,
  "criteria": {
    "<label>": <JSONContent or null>
  }
}
```

- `"type"` is always present with the value `"choice"`.
- `"criteria"` is always present and encodes the `Criteria` map as-is
  (a label with a nil description encodes as `null`).
- `"instructions"` is present only when `Instructions` is non-nil.

## Score

```go
type Score struct {
    Instructions JSONContent
    Criteria     []JSONContent
}
```

Score asks for a position on an ordered rubric. `Criteria[i]` describes score
`i`. Between 2 and 10 levels.

### (Score) MarshalJSON

```go
func (q Score) MarshalJSON() ([]byte, error)
```

MarshalJSON encodes the question in the API's wire format.

Wire shape:

```json
{
  "type": "score",
  "instructions": <JSONContent>,
  "criteria": [<JSONContent>, ...]
}
```

- `"type"` is always present with the value `"score"`.
- `"criteria"` is always present and encodes the `Criteria` slice as a JSON
  array, in index order.
- `"instructions"` is present only when `Instructions` is non-nil.

## RawQuestion

```go
type RawQuestion map[string]any
```

RawQuestion is sent to the API unchanged. It must carry a string `"type"`
field. It carries question fields this package does not model yet.

RawQuestion has no `MarshalJSON` method; it encodes with the standard
`encoding/json` map encoding, producing a JSON object of its keys and values
unchanged.
