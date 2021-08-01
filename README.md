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

Currently, only mongodb is supported out of the box (however it is very easy to extend the parser if needed).

# basic usage
```go
package main

import (

"github.com/rbicker/go-rsql"
"log"
)

func main(){
	parser, err := rsql.NewParser(rsql.Mongo())
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
	s := `status=="A",qty=lt=30`
	res, err := parser.Process(s)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "$or": [ { "status": "A" }, { "qty": { "$lt": 30 } } ] }
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


# advanced usage 

## custom operators
The library makes it easy to define custom operators:
```go
package main

import (

"fmt"
"github.com/rbicker/go-rsql"
"log"
)

func main(){
    // create custom operators for "exists"- and "all"-operations
    customOperators := []rsql.Operator{
        {
            Operator:       "=ex=",
            Formatter: func (key, value string) string {
                return fmt.Sprintf(`{ "%s": { "$exists": %s } }`, key, value)
            },
        },
        {
            Operator:       "=all=",
            Formatter: func(key, value string) string {
                return fmt.Sprintf(`{ "%s": { "$all": [ %s ] } }`, key, value[1:len(value)-1])
            },
        },
    }
    // create parser with default mongo operators
    // plus the two custom operators
    var opts []func(*rsql.Parser) error
    opts = append(opts, rsql.Mongo())
    opts = append(opts, rsql.WithOperators(customOperators...))
	parser, err := rsql.NewParser(opts...)
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
    // parse string with some default operators
    res, err := parser.Process(`(a==1;b==2),c=gt=5`)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "$or": [ { "$and": [ { "a": 1 }, { "b": 2 } ] }, { "c": { "$gt": 5 } } ] }
    
    // use custom operator =ex=
	res, err = parser.Process(`a=ex=true`)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "a": { "$exists": true } }
    
    // use custom list operator =all=
	res, err = parser.Process(`tags=all=('waterproof','rechargeable')`)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "tags": { "$all": [ 'waterproof','rechargeable' ] } }
}
```

## transform keys
If your database key naming scheme is different from the one used in your rsql statements, you can add functions to transform your keys.

```go
package main

import (
	"github.com/rbicker/go-rsql"
	"log"
	"strings"
)

func main() {
	transformer := func(s string) string {
		return strings.ToUpper(s)
	}
	parser, err := rsql.NewParser(rsql.Mongo(), rsql.WithKeyTransformers(transformer))
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
	s := `status=="a",qty=lt=30`
	res, err := parser.Process(s)
	if err != nil {
		log.Fatalf("error while parsing: %s", err)
	}
	log.Println(res)
	// { "$or": [ { "STATUS": "a" }, { "QTY": { "$lt": 30 } } ] }
}
```

## define allowed or forbidden keys
```go
package main

import (
	"github.com/rbicker/go-rsql"
	"log"
)

func main() {
	parser, err := rsql.NewParser(rsql.Mongo())
	if err != nil {
		log.Fatalf("error while creating parser: %s", err)
	}
	s := `status=="a",qty=lt=30`
	_, err = parser.Process(s, rsql.SetAllowedKeys([]string{"status, qty"}))
	// -> ok
	_, err = parser.Process(s, rsql.SetAllowedKeys([]string{"status"}))
	// -> error
	_, err = parser.Process(s, rsql.SetForbiddenKeys([]string{"status"}))
	// -> error
	_, err = parser.Process(s, rsql.SetAllowedKeys([]string{"age"}))
	// -> ok
}
```