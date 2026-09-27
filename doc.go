// Package typesafe is a client for the TypeSafe System One API.
//
// System One models such as Jev answer named, typed questions about a piece
// of state: a Noul returns the probability that a condition holds, a Choice
// selects one label from a set, and a Score places the state on an ordered
// rubric. Build a Client with NewClient, then call SystemOne with the state
// and a Questions map.
//
//	client, err := typesafe.NewClient() // reads TYPESAFE_API_KEY
//	res, err := client.SystemOne(ctx, "I was charged twice.", typesafe.Questions{
//	    "billing": typesafe.Noul{Instructions: "Is this about billing?"},
//	})
//	p := res.Nouls()["billing"].Noul
//
// The package depends only on the standard library. Tracing lives in the
// otel subpackage and is attached with WithInstrumentation.
package typesafe
