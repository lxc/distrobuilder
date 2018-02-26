package image

import (
	"log"
	"testing"

	pongo2 "gopkg.in/flosch/pongo2.v3"
)

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name       string
		context    pongo2.Context
		template   string
		expected   string
		shouldFail bool
	}{
		{
			"valid template",
			pongo2.Context{
				"foo": "bar",
			},
			"{{ foo }}",
			"bar",
			false,
		},
		{
			"variable not in context",
			pongo2.Context{},
			"{{ foo }}",
			"",
			false,
		},
		{
			"invalid template",
			pongo2.Context{
				"foo": nil,
			},
			"{{ foo }",
			"",
			true,
		},
		{
			"invalid context",
			pongo2.Context{
				"foo.bar": nil,
			},
			"{{ foo.bar }}",
			"",
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		ret, err := renderTemplate(tt.template, tt.context)
		if tt.shouldFail && err == nil {
			t.Fatal("test should have failed")
		}
		if ret != tt.expected {
			t.Fatalf("expected '%s', got '%s'", tt.expected, ret)
		}
	}
}
