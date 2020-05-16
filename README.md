go-rsql
=======

# overview
RSQL is a query language for parametrized filtering of entries in APIs. 
It is based on FIQL (Feed Item Query Language) – an URI-friendly syntax for expressing filters across the entries in an Atom Feed.
FIQL is great for use in URI; there are no unsafe characters, so URL encoding is not required.
On the other side, FIQL’s syntax is not very intuitive and URL encoding isn’t always that big deal,
so RSQL also provides a friendlier syntax for logical operators and some comparison operators.

This is a small RSQL helper library, written in golang.
It can be used to parse a RSQL string and turn it into a database query string.
Currently, only mongodb is supported.

# basic usage
```go
package main

import (

"github.com/rbicker/go-rsql"
"log"
)

func main(){
	parser, err := rsql.NewParser()
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
	s := `status=="A",qty=lt=30`
	res, err := parser.ToMongoQueryString(s)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "$or": [ { "status": { "$eq": "A" } }, { "qty": { "$lt": 30 } } ] }
}
```


# supported operators

The library supports the following basic operators by default:

| Basic Operator | Description         |
|----------------|---------------------|
| ==             | Equal To            |
| !=             | Not Equal To        |
| =gt=           | Greater Than        |
| =ge=           | Greater Or Equal To |
| =lt=           | Less Than           |
| =le=           | Less Or Equal To    |
| =in=           | In                  |
| =out=          | Not in              |

The following table lists two joining operators:

| Composite Operator | Description         |
|--------------------|---------------------|
| ;                  | Logical AND         |
| ,                  | Logical OR          |


# add custom operators
You can pass custom operators while creating a new parser:
```go
package main

import (

"github.com/rbicker/go-rsql"
"log"
)

func main(){
    // create a custom operator for "exists"- and "all"-operations
    customOperators := []rsql.Operator{
        {
            Operator:      "=ex=",
            MongoOperator: "$exists",
            ListType:      false,
        },
        {
            Operator:      "=all=",
            MongoOperator: "$all",
            ListType:      true,
        },
    }
    var opts []func(*rsql.Parser) error
    opts = append(opts, rsql.WithOperators(customOperators...))
	parser, err := rsql.NewParser(opts...)
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
    
    // test custom operator
	res, err := parser.ToMongoQueryString(`a=ex=true`)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "a": { "$exists": true } }
    
    // test custom list operator
	res, err = parser.ToMongoQueryString(`tags=all=('waterproof','rechargeable')`)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "tags": { "$all": [ 'waterproof','rechargeable' ] } }
}
```