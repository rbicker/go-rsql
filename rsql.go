package rsql

import (
	"fmt"
	"regexp"
	"strings"
)

// specialEncode is the map for encoding
// a list of special characters that could
// mess up the parser.
var specialEncode = map[string]string{
	`\(`: "%5C%28",
	`\)`: "%5C%29",
	`\,`: "%5C%2C",
	`\;`: "%5C%3B",
	`\=`: "%5C%3D",
}

// regex to match Operator within operation
var reOperator = regexp.MustCompile(`([!=])[^=]*=`)

// Operator represents a query Operator.
// It defines the Operator itself, the mongodb representation
// of the Operator and if it is a list Operator or not.
// Operators must match regex reOperator: `(!|=)[^=]*=`
type Operator struct {
	Operator      string
	MongoOperator string
	ListType      bool
}

// Parser represents a RSQL parser.
type Parser struct {
	operators []Operator
}

// NewParser returns a new rsql server.
func NewParser(options ...func(*Parser) error) (Parser, error) {
	// create parser
	var parser = Parser{}
	// run functional options
	for _, op := range options {
		err := op(&parser)
		if err != nil {
			return parser, fmt.Errorf("setting option failed: %w", err)
		}
	}
	var operators = []Operator{
		{
			"==",
			"$eq",
			false,
		},
		{
			"!=",
			"$ne",
			false,
		},
		{
			"=gt=",
			"$gt",
			false,
		},
		{
			"=ge=",
			"$gte",
			false,
		},
		{
			"=lt=",
			"$lt",
			false,
		},
		{
			"=le=",
			"$lte",
			false,
		},
		{
			"=in=",
			"$in",
			true,
		},
		{
			"=out=",
			"$nin",
			true,
		},
	}
	parser.operators = append(parser.operators, operators...)
	return parser, nil
}

// WithOperator adds custom operators to the parser
func WithOperators(operators ...Operator) func(parser *Parser) error {
	return func(parser *Parser) error {
		for _, o := range operators {
			if !reOperator.MatchString(o.Operator) {
				return fmt.Errorf("invalid Operator '%s' as it does not match regex `(!|=)[^=]*=`", o.Operator)
			}
		}
		parser.operators = append(parser.operators, operators...)
		return nil
	}
}

// ToMongoQueryString takes the given rsql string and converts it to a mongo query json string.
func (parser *Parser) ToMongoQueryString(s string) (string, error) {
	// url encode special strings
	s = encodeSpecial(s)
	if strings.Count(s, "(") != strings.Count(s, ")") {
		return "", fmt.Errorf("number of opening and closing parentheses don not match")
	}
	s, err := spreadParentheses(s)
	if err != nil {
		return "", fmt.Errorf("unable to spread parentheses: %w", err)
	}
	// handle operations
	ii, err := findOperations(s)
	if err != nil {
		return "", fmt.Errorf("error while looking for ii: %w", err)
	}
	// regex to match identifier within operation, before the equal or expression mark
	var reId = regexp.MustCompile(`^[^=!]+`)
	// regex to match value within the operation, after the equal sign
	var reValue = regexp.MustCompile(`[^=]+$`)
	// slices to store AND and OR parts
	var ors [][]string
	var ands []string
	// loop through operations
	for i, loc := range ii {
		operation := strings.Trim(s[loc[0]:loc[1]+1], " ")
		operator := reOperator.FindString(operation)
		id := reId.FindString(operation)
		value := reValue.FindString(operation)
		if operator == "" || id == "" || value == "" {
			return s, fmt.Errorf("incomplete operation '%s'", operation)
		}
		// parse operation
		var replacement string
		for _, op := range parser.operators {
			if operator == op.Operator {
				if op.ListType {
					if value[0:1] != "(" || value[len(value)-1:] != ")" {
						return "", fmt.Errorf("invalid or missing parentheses in list value '%s' in '%s'", value, operation)
					}
					replacement = fmt.Sprintf(`{ "%s": { "%s": [ %s ] } }`, id, op.MongoOperator, value[1:len(value)-1])
				} else {
					replacement = fmt.Sprintf(`{ "%s": { "%s": %s } }`, id, op.MongoOperator, value)
				}
				break
			}
		}
		if replacement == "" {
			return "", fmt.Errorf("unknown Operator '%s' in '%s'", operator, operation)
		}
		ands = append(ands, replacement)
		// when last index or if split by OR
		if i == len(ii)-1 || s[loc[1]+1:loc[1]+2] == "," {
			ors = append(ors, ands)
			ands = nil // reset
		}
	}
	// handle ANDs and ORs
	var res []string
	for _, ands := range ors {
		if len(ands) > 1 {
			res = append(res, fmt.Sprintf(`{ "$and": [ %s ] }`, strings.Join(ands, ", ")))
		} else {
			res = append(res, ands[0])
		}
	}
	switch len(res){
	case 0:
		s = "{ }"
	case 1:
		s = res[0]
	default:
		s = fmt.Sprintf(`{ "$or": [ %s ] }`, strings.Join(res, ", "))
	}
	// url decode
	s = decodeSpecial(s)
	return s, nil
}

// combine combines all the given string in a logical multiplication.
func combine(groups ...string) string {
	l := len(groups)
	// split task in groups of 2
	for l > 2 {
		sub := combine(groups[0:2]...)
		groups = append([]string{sub}, groups[2:]...)
		l = len(groups)
	}
	// if done
	if l == 1 {
		return groups[0]
	}
	// from this point on,
	// we are sure that we are dealing with
	// 2 groups
	elems := strings.Split(groups[0], ",")
	other := strings.Split(groups[1], ",")
	var comb []string // combinations
	// loop through first groups elements
	for i := 0; i < len(elems); i++ {
		e := elems[i]
		// loop through other groups elements
		for j := 0; j < len(other); j++ {
			o := other[j]
			comb = append(comb, fmt.Sprintf("%s;%s", e, o))
		}
	}
	return strings.Join(comb, ",")
}

// encodeSpecial encodes all the special strings
// that could get in the way of the parser.
func encodeSpecial(s string) string {
	for dec, enc := range specialEncode {
		s = strings.ReplaceAll(s, dec, enc)
	}
	return s
}

// decodeSpecial decodes all the special strings
// that could get in the way of the parser.
func decodeSpecial(s string) string {
	for dec, enc := range specialEncode {
		s = strings.ReplaceAll(s, enc, dec)
	}
	return s
}

// findOperations finds the locations of all the operations in the given string.
// Every location will have two integers, representing the start and end of the operation.
func findOperations(s string) ([][]int, error) {
	var res [][]int
	start := 0
	var list bool
	var before string
	runes := []rune(s)
	for i, r := range runes {
		c := string(r)
		// handle lists
		if c == "(" && list {
			return nil, fmt.Errorf("found nested parentheses in list while parsing '%s'", s)
		}
		if c == "(" && before == "=" {
			list = true
		}
		if c == ")" && list {
			list = false
		}
		// found operation
		if c == ";" || (c == "," && !list) {
			if i == 0 {
				return nil, fmt.Errorf("given string '%s' starts with '%s'", s, c)
			}
			res = append(res, []int{start, i - 1})
			start = i + 1
		}
		// remember the current character for the next iteration
		before = c
	}
	end := len(s) - 1
	if start < end {
		res = append(res, []int{start, len(s) - 1})
	}
	return res, nil
}

// findORs finds the locations of all OR blocks in the given string.
// Every location will have two integers, representing the start and end of the block.
// If n is greater than 0, n locations (from the left) are returned at most.
func findORs(s string, n int) ([][]int, error) {
	if n == 0 {
		return nil, nil
	}
	var res [][]int
	var list bool
	var before string
	start, found := 0, 0
	runes := []rune(s)
	for i, r := range runes {
		c := string(r)
		// handle lists
		if c == "(" && list {
			return nil, fmt.Errorf("found nested parentheses in list while parsing '%s'", s)
		}
		if c == "(" && before == "=" {
			list = true
		}
		if c == ")" && list {
			list = false
		}
		// found OR
		if c == "," && !list {
			if i == 0 {
				return nil, fmt.Errorf("given string '%s' starts with a comma", s)
			}
			res = append(res, []int{start, i - 1})
			start = i + 1
			found += 1
			if n > 0 && found == n {
				return res, nil
			}
		}
		// remember the current character for the next iteration
		before = c
	}
	res = append(res, []int{start, len(s) - 1})
	return res, nil
}

// findOuterParentheses finds indexes of opening and closing parentheses.
// Every entry will have two integers, the first one providing the index of the
// opening parentheses, the second one the index of the closing parentheses.
func findOuterParentheses(s string, n int) ([][]int, error) {
	var res [][]int
	if strings.Count(s, "(") != strings.Count(s, ")") {
		return nil, fmt.Errorf("number of opening and closing parentheses dont match")
	}
	start := -1
	var countFound, countOpening, countClosing int
	var list bool
	var before string
	runes := []rune(s)
	for i, r := range runes {
		c := string(r)
		// found opening
		if c == "(" {
			if list {
				return nil, fmt.Errorf("found nested parentheses in list while parsing '%s'", s)
			}
			if before != "=" {
				if start < 0 {
					start = i
				}
				countOpening += 1
			} else {
				list = true
			}
		}
		// found closing
		if c == ")" && start >= 0 {
			if list {
				list = false
			} else {
				countClosing += 1
			}
		}
		// if outer parentheses found
		if start >= 0 && countOpening == countClosing {
			res = append(res, []int{start, i})
			start = -1
			countOpening = 0
			countClosing = 0
			countFound += 1
		}
		// if we found enough matching parentheses
		if n > 0 && countFound == n {
			return res, nil
		}
		// remember the current character for the next iteration
		before = c
	}
	return res, nil
}

// spreadParentheses resolves all the parentheses within the given string
// and returns the expanded version.
func spreadParentheses(s string) (string, error) {
	parentheses, err := findOuterParentheses(s, -1)
	if err != nil {
		return "", fmt.Errorf("unable to determine parentheses: %w", err)
	}
	// handle nested parentheses first
	offset := 0
	for _, p := range parentheses {
		start, end := p[0]+offset, p[1]+offset
		content := s[start+1 : end]
		nested, err := findOuterParentheses(content, 1)
		if err != nil {
			return "", fmt.Errorf("unable to determine parentheses: %w", err)
		}
		if len(nested) > 0 {
			replacement, err := spreadParentheses(content)
			if err != nil {
				return s, err
			}
			before, after := "", ""
			if start > 0 {
				before = s[:start]
			}
			if end < len(s)-1 {
				after = s[end:]
			}
			l := len(s)
			s = before + "(" + replacement + ")" + after
			offset = len(s) - l
		}
	}
	// from here on, there are no more nested parentheses
	// we need this for-loop because this function might add
	// parentheses while processing the string, we do not
	// want to miss those, this is why we do not loop over
	// the result of findOuterParentheses directly
	for {
		parentheses, err = findOuterParentheses(s, 1)
		if err != nil {
			return "", fmt.Errorf("unable to determine parentheses: %w", err)
		}
		if len(parentheses) == 0 {
			// we are done
			return s, nil
		}
		// indexes of our parentheses
		start, end := parentheses[0][0], parentheses[0][1]
		// indexes for our replacement
		before, after := start, end+1
		// groups for the combination
		var groups []string
		// whether we need to add parentheses to our result or not
		var addParentheses bool
		// look at the statement before
		// the parentheses
		for i := start; i-1 > 0; i-- {
			// switch character before "("
			c := s[i-1 : i]
			// skip whitespace
			if c == " " {
				continue
			}
			switch c {
			case ";":
				// for "AND", look at what comes
				// before the and
				for j := i - 1; j > 0; j-- {
					cc := s[j : j+1]
					// skip whitespace
					if cc == " " {
						continue
					}
					locations, err := findORs(s[0:j], -1)
					if err != nil {
						return "", fmt.Errorf("unable to split string '%s' into OR-blocks: %w", s, err)
					}
					if len(locations) == 0 {
						return "", fmt.Errorf("invalid rsql statement, empty 'AND' or 'OR' statement in '%s'", s)
					}
					before = locations[len(locations)-1][0]
					groups = append(groups, s[before:j])
					break
				}
			case ",":
				// nothing to do for "OR"
			default:
				return "", fmt.Errorf("unexpected character before parentheses %s: %s", s[start:end], c)
			}
			break
		}
		// add content to groups
		content := s[start+1 : end]
		groups = append(groups, content)
		// look at the statement after
		// the parentheses
		for i := end + 1; i < len(s); i++ {
			// switch character after ")"
			c := s[i : i+1]
			// skip whitespace
			if c == " " {
				continue
			}
			switch c {
			case ";":
				// for "AND", look at what comes
				// after the and
				for j := i + 1; j < len(s); j++ {
					cc := s[j : j+1]
					// skip whitespace
					if cc == " " {
						continue
					}
					if cc == "(" {
						// we will need to add parentheses to our result
						addParentheses = true
						// look for closing parentheses
						for k := j + 1; k < len(s); k++ {
							ccc := s[k : k+1]
							if ccc == ")" {
								after = k + 1
								groups = append(groups, s[j+1:k])
								break
							}
						}
					} else {
						locations, err := findORs(s, 1)
						if err != nil {
							return "", fmt.Errorf("unable to split string '%s' into OR-blocks: %w", s, err)
						}
						if len(locations) == 0 {
							return "", fmt.Errorf("invalid rsql statement, empty 'AND' or 'OR' statement in '%s'", s)
						}
						after = j + locations[0][1]
						groups = append(groups, s[j:after])
					}
					break
				}
			case ",":
				// nothing to do for "or"
			default:
				return "", fmt.Errorf("unexpected character before parentheses %s: %s", s[start:end], c)
			}
			break
		}
		// combine all groups
		res := combine(groups...)
		if addParentheses {
			res = "(" + res + ")"
		}
		// add result
		s = s[:before] + res + s[after:]
	}
}
