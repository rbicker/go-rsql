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
var reOperator = regexp.MustCompile(`([!=])[^=()]*=`)

// Operator represents a query Operator.
// It defines the Operator itself, the mongodb representation
// of the Operator and if it is a list Operator or not.
// Operators must match regex reOperator: `(!|=)[^=()]*=`
type Operator struct {
	Operator  string
	Formatter func(key, value string) string
}

// Parser represents a RSQL parser.
type Parser struct {
	operators       []Operator
	andFormatter    func(ss []string) string
	orFormatter     func(ss []string) string
	keyTransformers []func(s string) string
}

// NewParser returns a new rsql server.
func NewParser(options ...func(*Parser) error) (*Parser, error) {
	// create parser
	var parser = Parser{}
	// run functional options
	for _, op := range options {
		err := op(&parser)
		if err != nil {
			return nil, fmt.Errorf("setting option failed: %w", err)
		}
	}
	if parser.andFormatter == nil {
		return nil, fmt.Errorf("AND-formatter is not defined")
	}
	if parser.orFormatter == nil {
		return nil, fmt.Errorf("OR-formatter is not defined")
	}
	return &parser, nil
}

// Mongo adds the default mongo operators to the parser
func Mongo() func(parser *Parser) error {
	return func(parser *Parser) error {
		// operators
		var operators = []Operator{
			{
				"==",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$eq": %s } }`, key, value)
				},
			},
			{
				"!=",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$ne": %s } }`, key, value)
				},
			},
			{
				"=gt=",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$gt": %s } }`, key, value)
				},
			},
			{
				"=ge=",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$gte": %s } }`, key, value)
				},
			},
			{
				"=lt=",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$lt": %s } }`, key, value)
				},
			},
			{
				"=le=",
				func(key, value string) string {
					return fmt.Sprintf(`{ "%s": { "$lte": %s } }`, key, value)
				},
			},
			{
				"=in=",
				func(key, value string) string {
					// remove parentheses
					value = value[1 : len(value)-1]
					return fmt.Sprintf(`{ "%s": { "$in": %s } }`, key, value)
				},
			},
			{
				"=out=",
				func(key, value string) string {
					// remove parentheses
					value = value[1 : len(value)-1]
					return fmt.Sprintf(`{ "%s": { "$nin": %s } }`, key, value)
				},
			},
		}
		parser.operators = append(parser.operators, operators...)
		// AND formatter
		parser.andFormatter = func(ss []string) string {
			if len(ss) > 1 {
				return fmt.Sprintf(`{ "$and": [ %s ] }`, strings.Join(ss, ", "))
			}
			if len(ss) == 0 {
				return ""
			}
			return ss[0]
		}
		// OR formatter
		parser.orFormatter = func(ss []string) string {
			if len(ss) > 1 {
				return fmt.Sprintf(`{ "$or": [ %s ] }`, strings.Join(ss, ", "))
			}
			if len(ss) == 0 {
				return "{ }"
			}
			return ss[0]
		}
		return nil
	}
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

// WithKeyTransformers adds functions to alter key names in any way.
func WithKeyTransformers(transformers ...func(string) string) func(parser *Parser) error {
	return func(parser *Parser) error {
		parser.keyTransformers = append(parser.keyTransformers, transformers...)
		return nil
	}
}

// ProcessOptions contains options for the parser's Process function.
type ProcessOptions struct {
	allowedKeys   []string
	forbiddenKeys []string
}

// SetAllowedKeys set's the keys which can be used for querying.
func SetAllowedKeys(keys []string) func(opts *ProcessOptions) error {
	return func(opts *ProcessOptions) error {
		opts.allowedKeys = keys
		return nil
	}
}

// SetForbiddenKeys set's the keys which must not be used for querying.
func SetForbiddenKeys(keys []string) func(opts *ProcessOptions) error {
	return func(opts *ProcessOptions) error {
		opts.forbiddenKeys = keys
		return nil
	}
}

// containsString checks if a given slice of strings contains a given string.
func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// Process takes the given string and processes it using parser's operators.
func (parser *Parser) Process(s string, options ...func(*ProcessOptions) error) (string, error) {
	// set process options
	opts := ProcessOptions{}
	for _, op := range options {
		err := op(&opts)
		if err != nil {
			return "", fmt.Errorf("setting process option failed: %w", err)
		}
	}
	// regex to match identifier within operation, before the equal or expression mark
	var reKey = regexp.MustCompile(`^[^=!]+`)
	// regex to match value within the operation, after the equal sign
	var reValue = regexp.MustCompile(`[^=]+$`)
	// get ORs
	locations, err := findORs(s, -1)
	if err != nil {
		return "", fmt.Errorf("unable to find ORs: %w", err)
	}
	var ors []string
	for _, loc := range locations {
		start, end := loc[0], loc[1]
		content := s[start:end]
		// handle ANDs
		locs, err := findANDs(content, -1)
		if err != nil {
			return "", fmt.Errorf("unable to find ANDs: %w", err)
		}
		var ands []string
		for _, l := range locs {
			start, end = l[0], l[1]
			content := content[start:end]
			// handle parentheses
			parentheses, err := findOuterParentheses(content, -1)
			if err != nil {
				return "", fmt.Errorf("unable to find parentheses: %w", err)
			}
			for _, p := range parentheses {
				start, end := p[0], p[1]
				content := content[start+1 : end]
				// handle nested
				replacement, err := parser.Process(content)
				if err != nil {
					return "", err
				}
				ands = append(ands, replacement)
			}
			if len(parentheses) > 0 {
				// location is already fully handled
				continue
			}
			// if no parentheses, it should be an operation
			operator := reOperator.FindString(content)
			key := reKey.FindString(content)
			value := reValue.FindString(content)
			if operator == "" || key == "" || value == "" {
				return "", fmt.Errorf("incomplete operation '%s'", content)
			}
			// run key transformers
			for _, t := range parser.keyTransformers {
				key = t(key)
			}
			// check if key is allowed
			if containsString(opts.forbiddenKeys, key) {
				return "", fmt.Errorf("given key '%s' is not allowed", key)
			}
			if len(opts.allowedKeys) > 0 && !containsString(opts.allowedKeys, key) {
				return "", fmt.Errorf("given key '%s' is not allowed", key)
			}
			// parse operation
			var res string
			for _, op := range parser.operators {
				if operator == op.Operator {
					res = op.Formatter(key, value)
					break
				}
			}
			if res == "" {
				return "", fmt.Errorf("unknown operator '%s' in '%s'", operator, content)
			}
			ands = append(ands, res)
		}
		// replacement for AND-block
		replacement := parser.andFormatter(ands)
		ors = append(ors, replacement)
	}
	// replace OR-block and return
	return parser.orFormatter(ors), nil
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

// findParts finds the locations of separated blocks while considering parentheses.
// If n is greater than 0, n parts (from the left) are returned at most.
func findParts(s string, n int, separators ...string) ([][]int, error) {
	if len(s) == 0 {
		return nil, nil
	}
	// validations
	if len(separators) == 0 {
		return nil, fmt.Errorf("no separators given")
	}
	for _, sep := range separators {
		if s[0:1] == sep {
			return nil, fmt.Errorf("given string starts with a separators")
		}
		if s[len(s)-1:] == sep {
			return nil, fmt.Errorf("given string ends with a separators")
		}
	}
	var res [][]int
	var start, par, found int
	runes := []rune(s)
	for i, r := range runes {
		c := string(r)
		// parentheses
		if c == "(" {
			par++
		}
		if c == ")" {
			par--
			if par < 0 {
				return nil, fmt.Errorf("parentheses mismatch")
			}
		}
		// while par for parentheses is not zero,
		// don't bother checking if separators was found
		if par == 0 {
			for _, sep := range separators {
				if c == sep {
					// found part
					found++
					res = append(res, []int{start, i})
					start = i + 1
					if n > 0 && found == n {
						// return if found enough parts
						return res, nil
					}
				}
			}

		}
	}
	// append part after last separators
	end := len(s)
	if start < end {
		res = append(res, []int{start, end})
	}
	return res, nil
}

// findORs finds the locations of all OR blocks in the given string.
// Every location will have two integers, representing the start and end of the block.
// If n is greater than 0, n locations (from the left) are returned at most.
func findORs(s string, n int) ([][]int, error) {
	return findParts(s, n, ",")
}

// findANDs finds the locations of all ANDs blocks in the given string.
// Every location will have two integers, representing the start and end of the block.
// If n is greater than 0, n locations (from the left) are returned at most.
func findANDs(s string, n int) ([][]int, error) {
	return findParts(s, n, ";")
}

// findOuterParentheses finds indexes of opening and closing parentheses.
// Every entry will have two integers, the first one providing the index of the
// opening parentheses, the second one the index of the closing parentheses.
func findOuterParentheses(s string, n int) ([][]int, error) {
	if strings.Count(s, "(") != strings.Count(s, ")") {
		return nil, fmt.Errorf("number of opening and closing parentheses dont match in string '%s'", s)
	}
	var res [][]int
	var start, par, nested, found int
	var op bool
	runes := []rune(s)
	for i, r := range runes {
		c := string(r)
		// start or part of operator
		if c == "=" || c == "!" {
			op = true
		}
		// end of operation
		if (c == "," || c == ";") && nested == 0 {
			op = false
		}
		// opening
		if c == "(" {
			if op {
				nested++
			} else {
				if par == 0 {
					start = i
				}
				par++
			}
		}
		// closing
		if c == ")" {
			if nested > 0 {
				nested--
				if nested < 0 {
					return nil, fmt.Errorf("parentheses mismatch")
				}
				continue
			} else {
				par--
			}
			if par > 0 {
				// we need to find more
				continue
			}
			if par < 0 {
				return nil, fmt.Errorf("parentheses mismatch")
			}
			// found outer parentheses
			found++
			op = false
			res = append(res, []int{start, i})
			start = i + 1
			if n > 0 && found == n {
				// return if found enough parts
				return res, nil
			}

		}
	}
	return res, nil
}
