# Experimental streams & promises in Go

This library is an experiment in promise- and stream-style processing in Go. 

This effort was inspired by the tiresome conventional handling of errors in Go, e.g. `if err != nil { return nil, err }`.

The unifying characteristics of streams and promises, yielding this attempt at a single library, are the use of functional style, and the single layer of error handling.

## Promises

This library attempts to follow the semantics of Javascript's [Promises/A+](https://promisesaplus.com) standard.


## Streams

This library facilitates iterative "pipelines" (or "flows") of transformation functions, which are executed on an input series (similar to "map()").
