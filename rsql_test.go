package rsql

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_encodeSpecial(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "encode special strings",
			s:    `(x==1;y=='\(hello\, how are you?\;thanks\)')`,
			want: "(x==1;y=='%5C%28hello%5C%2C how are you?%5C%3Bthanks%5C%29')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeSpecial(tt.s); got != tt.want {
				t.Errorf("encodeSpecial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeSpecial(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "decode special strings",
			s:    "(x==1;y=='%5C%28hello%5C%2C how are you?%5C%3Bthanks%5C%29')",
			want: `(x==1;y=='\(hello\, how are you?\;thanks\)')`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeSpecial(tt.s); got != tt.want {
				t.Errorf("decodeSpecial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findOuterParentheses(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		n       int
		want    [][]int
		wantErr bool
	}{
		{
			name:    "parentheses not matching",
			s:       "(x)(a(b)",
			wantErr: true,
		},
		{
			name: "one",
			s:    "(x==1)",
			want: [][]int{
				{
					0,
					5,
				},
			},
		},
		{
			name: "two",
			s:    "(x==1),(y==1)",
			want: [][]int{
				{
					0,
					5,
				},
				{
					7,
					12,
				},
			},
		},
		{
			name: "object id",
			s:    `(a==1),(_id=in=(ObjectId("xxx"),ObjectId("yyy")))`,
			want: [][]int{
				{
					0,
					5,
				},
				{
					7,
					48,
				},
			},
		},
		{
			name: "two, but only look for one",
			s:    "(x==1),(y==1)",
			n:    1,
			want: [][]int{
				{
					0,
					5,
				},
			},
		},
		{
			name: "containing list",
			s:    "(y==2),x=in=(1,2,3)",
			want: [][]int{
				{
					0,
					5,
				},
			},
		},
		{
			name: "containing nested list",
			s:    "(y==2),x=in=(1,(2),3)",
			want: [][]int{
				{
					0,
					5,
				},
			},
		},
		{
			name: "at beginning",
			s:    "(b==1,c==1);a==1",
			want: [][]int{
				{
					0,
					10,
				},
			},
		},
		{
			name: "at end",
			s:    "a==1;(b==1,c==1)",
			want: [][]int{
				{
					5,
					15,
				},
			},
		},
		{
			name: "nested",
			s:    "((a==1,a==2),(b==1,b==2))",
			want: [][]int{
				{
					0,
					24,
				},
			},
		},
		{
			name: "nested part 2",
			s:    "(a==1,a==2),(b==1,b==2)",
			want: [][]int{
				{
					0,
					10,
				},
				{
					12,
					22,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findOuterParentheses(tt.s, tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("findOuterParentheses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOuterParentheses() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_ProcessOptions(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		opts    []func(*ProcessOptions) error
		wantErr bool
	}{
		{
			name:    "all keys allowed",
			s:       "a==1",
			opts:    []func(*ProcessOptions) error{},
			wantErr: false,
		},
		{
			name: "key allowed",
			s:    "a==1",
			opts: []func(*ProcessOptions) error{
				SetAllowedKeys([]string{"a"}),
			},
			wantErr: false,
		},
		{
			name: "key not allowed",
			s:    "a==1",
			opts: []func(*ProcessOptions) error{
				SetAllowedKeys([]string{"b"}),
			},
			wantErr: true,
		},
		{
			name: "key forbidden",
			s:    "a==1",
			opts: []func(*ProcessOptions) error{
				SetForbiddenKeys([]string{"a"}),
			},
			wantErr: true,
		},
		{
			name: "key not forbidden",
			s:    "a==1",
			opts: []func(*ProcessOptions) error{
				SetForbiddenKeys([]string{"b"}),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []func(*Parser) error
			opts = append(opts, Mongo())
			parser, err := NewParser(opts...)
			if err != nil {
				t.Fatalf("error while creating parser: %s", err)
			}
			_, err = parser.Process(tt.s, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestParser_ProcessMongo(t *testing.T) {
	tests := []struct {
		name            string
		s               string
		customOperators []Operator
		want            string
		wantErr         bool
	}{
		{
			name: "empty",
			s:    "",
			want: "{ }",
		},
		{
			name: "==",
			s:    "a==1",
			want: `{ "a": { "$eq": 1 } }`,
		},
		{
			name: "!=",
			s:    "a!=1",
			want: `{ "a": { "$ne": 1 } }`,
		},
		{
			name: "=gt=",
			s:    "a=gt=1",
			want: `{ "a": { "$gt": 1 } }`,
		},
		{
			name: "=ge=",
			s:    "a=ge=1",
			want: `{ "a": { "$gte": 1 } }`,
		},
		{
			name: "=lt=",
			s:    "a=lt=1",
			want: `{ "a": { "$lt": 1 } }`,
		},
		{
			name: "=le=",
			s:    "a=le=1",
			want: `{ "a": { "$lte": 1 } }`,
		},
		{
			name: "=in=",
			s:    "a=in=(1,2,3)",
			want: `{ "a": { "$in": 1,2,3 } }`,
		},
		{
			name: "=out=",
			s:    "a=out=(1,2,3)",
			want: `{ "a": { "$nin": 1,2,3 } }`,
		},
		{
			name: "(a==1)",
			s:    "(a==1)",
			want: `{ "a": { "$eq": 1 } }`,
		},
		{
			name: "a==1;b==2",
			s:    "a==1;b==2",
			want: `{ "$and": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 2 } } ] }`,
		},
		{
			name: "a==1,b==2",
			s:    "a==1,b==2",
			want: `{ "$or": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 2 } } ] }`,
		},
		{
			name: "a==1;b==2,c==1",
			s:    "a==1;b==2,c==1",
			want: `{ "$or": [ { "$and": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 2 } } ] }, { "c": { "$eq": 1 } } ] }`,
		},
		{
			name: "(a==1;b==2),c=gt=5",
			s:    "(a==1;b==2),c=gt=5",
			want: `{ "$or": [ { "$and": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 2 } } ] }, { "c": { "$gt": 5 } } ] }`,
		},
		{
			name: "c==1,(a==1;b==2)",
			s:    "c==1,(a==1;b==2)",
			want: `{ "$or": [ { "c": { "$eq": 1 } }, { "$and": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 2 } } ] } ] }`,
		},
		{
			name: "a==1;(b==1,c==2)",
			s:    "a==1;(b==1,c==2)",
			want: `{ "$and": [ { "a": { "$eq": 1 } }, { "$or": [ { "b": { "$eq": 1 } }, { "c": { "$eq": 2 } } ] } ] }`,
		},
		{
			name: "(a==1,b==1);(c==1,d==2)",
			s:    "(a==1,b==1);(c==1,d==2)",
			want: `{ "$and": [ { "$or": [ { "a": { "$eq": 1 } }, { "b": { "$eq": 1 } } ] }, { "$or": [ { "c": { "$eq": 1 } }, { "d": { "$eq": 2 } } ] } ] }`,
		},
		{
			name: "custom operator: =ex=",
			s:    "a=ex=true",
			customOperators: []Operator{
				{
					Operator: "=ex=",
					Formatter: func(key, value string) string {
						return fmt.Sprintf(`{ "%s": { "$exists": %s } }`, key, value)
					},
				},
			},
			want: `{ "a": { "$exists": true } }`,
		},
		{
			name: "custom list operator: =all=",
			s:    "tags=all=('waterproof','rechargeable')",
			customOperators: []Operator{
				{
					Operator: "=all=",
					Formatter: func(key, value string) string {

						return fmt.Sprintf(`{ "%s": { "$all": [ %s ] } }`, key, value[1:len(value)-1])
					},
				},
			},
			want: `{ "tags": { "$all": [ 'waterproof','rechargeable' ] } }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []func(*Parser) error
			opts = append(opts, Mongo())
			opts = append(opts, WithOperators(tt.customOperators...))
			parser, err := NewParser(opts...)
			if err != nil {
				t.Fatalf("error while creating parser: %s", err)
			}
			got, err := parser.Process(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Process() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findParts(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		n          int
		separators []string
		want       [][]int
		wantErr    bool
	}{
		{
			name: "empty",
			s:    "",
			n:    -1,
		},
		{
			name:    "no separators",
			s:       "(a==1),(b==1)",
			n:       -1,
			wantErr: true,
		},
		{
			name:       "start with separators",
			s:          ",(a==1),(b==1)",
			n:          -1,
			separators: []string{","},
			wantErr:    true,
		},
		{
			name:       "end with separators",
			s:          "(a==1),(b==1),",
			n:          -1,
			separators: []string{","},
			wantErr:    true,
		},
		{
			name:       "parentheses mismatch",
			s:          "(a==1)),(b==1)",
			n:          -1,
			separators: []string{","},
			wantErr:    true,
		},
		{
			name:       "parentheses mismatch in operation",
			s:          "a=in=(1),2,3)",
			n:          -1,
			separators: []string{","},
			wantErr:    true,
		},
		{
			name:       "nested",
			s:          "((a==1),(b==1)),(c==1)",
			n:          -1,
			separators: []string{","},
			want: [][]int{
				{
					0,
					15,
				},
				{
					16,
					22,
				},
			},
		},
		{
			name:       "return one",
			s:          "(a==1),(b==1)",
			n:          1,
			separators: []string{","},
			want: [][]int{
				{
					0,
					6,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findParts(tt.s, tt.n, tt.separators...)
			if (err != nil) != tt.wantErr {
				t.Errorf("findParts = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findParts() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findORs(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		n       int
		want    [][]int
		wantErr bool
	}{
		{
			name: "simple or",
			s:    "(a==1),(b==1)",
			n:    -1,
			want: [][]int{
				{
					0,
					6,
				},
				{
					7,
					13,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findORs(tt.s, tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("findORs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findORs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findANDs(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		n       int
		want    [][]int
		wantErr bool
	}{
		{
			name: "simple and",
			s:    "(a==1);(b==1)",
			n:    -1,
			want: [][]int{
				{
					0,
					6,
				},
				{
					7,
					13,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findANDs(tt.s, tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("findANDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findANDs() got = %v, want %v", got, tt.want)
			}
		})
	}
}
