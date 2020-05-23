package rsql

import (
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

func Test_spreadParentheses(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    string
		wantErr bool
	}{
		{
			name: "remove all unnecessary parentheses",
			s:    "((a==1,a==2),(b==1,b==2))",
			want: "a==1,a==2,b==1,b==2",
		},
		{
			name: "spread simple - 1",
			s:    "a==1;(b==1,c==1)",
			want: "a==1;b==1,a==1;c==1",
		},
		{
			name: "spread simple - 2",
			s:    "(b==1,c==1);a==1",
			want: "b==1;a==1,c==1;a==1",
		},
		{
			name: "spread two groups",
			s:    "(a==1,b==1);(c==1,d==1)",
			want: "a==1;c==1,a==1;d==1,b==1;c==1,b==1;d==1",
		},
		{
			name: "spread three groups",
			s:    "(a==1,b==1);(c==1,d==1);(e==1,f==1)",
			want: "a==1;c==1;e==1,a==1;c==1;f==1,a==1;d==1;e==1,a==1;d==1;f==1,b==1;c==1;e==1,b==1;c==1;f==1,b==1;d==1;e==1,b==1;d==1;f==1",
		},
		{
			name: "spread ands and ors",
			s:    "a==1;(b==1,c==1),d==1",
			want: "a==1;b==1,a==1;c==1,d==1",
		},
		{
			name: "spread nested",
			s:    "a==1;(b==1;(c==1;d==1,e==1))",
			want: "a==1;b==1;c==1;d==1,a==1;b==1;e==1",
		},
		{
			name: "spread while containing nested in operation",
			s:    "a==1;(x=in=(1,2),b==1)",
			want: "a==1;x=in=(1,a==1;2),a==1;b==1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := spreadParentheses(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("spreadParentheses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("spreadParentheses() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_combine(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		want   string
	}{
		{
			name: "one group with one element",
			groups: []string{
				"a",
			},
			want: "a",
		},
		{
			name: "two groups with one element each",
			groups: []string{
				"a",
				"b",
			},
			want: "a;b",
		},
		{
			name: "three groups with one element each",
			groups: []string{
				"a",
				"b",
				"c",
			},
			want: "a;b;c",
		},
		{
			name: "two groups with two elements each",
			groups: []string{
				"a,b",
				"c,d",
			},
			want: "a;c,a;d,b;c,b;d",
		},
		{
			name: "three groups with two elements each",
			groups: []string{
				"a,b",
				"c,d",
				"e,f",
			},
			want: "a;c;e,a;c;f,a;d;e,a;d;f,b;c;e,b;c;f,b;d;e,b;d;f",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := combine(tt.groups...); got != tt.want {
				t.Errorf("combine() = %v, want %v", got, tt.want)
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
			s:    "(x)",
			want: [][]int{
				{
					0,
					2,
				},
			},
		},
		{
			name: "two",
			s:    "(x)(y)",
			want: [][]int{
				{
					0,
					2,
				},
				{
					3,
					5,
				},
			},
		},
		{
			name: "two, but only look for one",
			s:    "(x)(y)",
			n:    1,
			want: [][]int{
				{
					0,
					2,
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
			name:    "containing invalid list",
			s:       "(y==2),x=in=(1,(2),3)",
			wantErr: true,
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

func TestParser_ToMongoQueryString(t *testing.T) {
	tests := []struct {
		name            string
		s               string
		customOperators []Operator
		want            string
		wantErr         bool
	}{
		{
			name:    "parentheses not matching",
			s:       "(a==1",
			wantErr: true,
		},
		{
			name:    "parentheses not matching with escaping",
			s:       `(a=='this is what I am looking for \)`,
			wantErr: true,
		},
		{
			name:    "invalid complex Operator",
			s:       "a=in='x','y','z'",
			wantErr: true,
		},
		{
			name: "empty filter",
			s: "",
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
			name: "complex query",
			s:    `status=="A",qty=lt=30`,
			want: `{ "$or": [ { "status": { "$eq": "A" } }, { "qty": { "$lt": 30 } } ] }`,
		},
		{
			name: "custom Operator: =ex=",
			s:    "a=ex=true",
			customOperators: []Operator{
				{
					Operator:      "=ex=",
					MongoOperator: "$exists",
					ListType:      false,
				},
			},
			want: `{ "a": { "$exists": true } }`,
		},
		{
			name: "custom list Operator: =all=",
			s:    "tags=all=('waterproof','rechargeable')",
			customOperators: []Operator{
				{
					Operator:      "=all=",
					MongoOperator: "$all",
					ListType:      true,
				},
			},
			want: `{ "tags": { "$all": [ 'waterproof','rechargeable' ] } }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []func(*Parser) error
			opts = append(opts, WithOperators(tt.customOperators...))
			parser, _ := NewParser(opts...)
			got, err := parser.ToMongoQueryString(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToMongoQueryString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToMongoQueryString() got = %v, want %v", got, tt.want)
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
			name: "n is zero",
			s:    "(a==1),(b==1)",
			n:    0,
			want: nil,
		},
		{
			name:    "start string with a comma",
			s:       ",(a==1),(b==1)",
			n:       1,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nested parentheses in list operation",
			s:       "(a==1),(b=in=(1,(2),3))",
			n:       -1,
			want:    nil,
			wantErr: true,
		},
		{
			name: "simple or",
			s:    "(a==1),(b==1)",
			n:    -1,
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
			name: "list operation",
			s:    "(a==1),(b=in=(1,2,3))",
			n:    -1,
			want: [][]int{
				{
					0,
					5,
				},
				{
					7,
					20,
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

func Test_findOperations(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		want    [][]int
		wantErr bool
	}{
		{
			name:    "start with split Operator",
			s:       ";a==1,b==2",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nested parentheses in list",
			s:       "a==1,b=in=(1,(2),3)",
			want:    nil,
			wantErr: true,
		},
		{
			name: "simple",
			s:    "a==1,b!=2",
			want: [][]int{
				{
					0,
					3,
				},
				{
					5,
					8,
				},
			},
		},
		{
			name: "list",
			s:    "a==1,b=in=(1,2,3)",
			want: [][]int{
				{
					0,
					3,
				},
				{
					5,
					16,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findOperations(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("findOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOperations() got = %v, want %v", got, tt.want)
			}
		})
	}
}
