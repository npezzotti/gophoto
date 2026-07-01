package template

import (
	"bytes"
	"io"
	"testing"
)

func TestNewTemplateCache(t *testing.T) {
	tests := []struct {
		name         string
		pagesGlob    string
		partialsGlob string
		baseTemplate string
		wantKeys     []string
		wantLen      int
		wantErr      bool
	}{
		{
			name:         "successful template cache creation",
			pagesGlob:    "testdata/pages/*.html",
			partialsGlob: "testdata/partials/*.html",
			baseTemplate: "testdata/base.html",
			wantKeys:     []string{"test_page.html"},
			wantLen:      1,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := NewTemplateCache(tt.pagesGlob, tt.partialsGlob, tt.baseTemplate)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("unexpected error: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("expected error but got none")
			}

			if len(got) != tt.wantLen {
				t.Fatalf("got length %d, expected %d", len(got), tt.wantLen)
			}

			for _, key := range tt.wantKeys {
				if got[key] == nil {
					t.Errorf("expected key %q to be present in the cache", key)
				}
			}
		})
	}
}

func TestTemplateCache_RenderTemplate(t *testing.T) {
	tests := []struct {
		name string
		// Named input parameters for receiver constructor.
		pagesGlob    string
		partialsGlob string
		baseTemplate string
		w            io.Writer
		tmpl         string
		data         any
		wantErr      bool
	}{
		{
			name:         "successful template rendering",
			pagesGlob:    "testdata/pages/*.html",
			partialsGlob: "testdata/partials/*.html",
			baseTemplate: "testdata/base.html",
			w:            &bytes.Buffer{},
			tmpl:         "test_page.html",
			data:         "Test Data",
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := NewTemplateCache(tt.pagesGlob, tt.partialsGlob, tt.baseTemplate)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			gotErr := tc.RenderTemplate(tt.w, tt.tmpl, tt.data)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("unexpected error: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("template rendering succeeded unexpectedly")
			}
			// Optionally, you can check the output in the buffer
			output := tt.w.(*bytes.Buffer).String()
			if output == "" {
				t.Errorf("output is empty")
			}
			if !bytes.Contains([]byte(output), []byte(tt.data.(string))) {
				t.Errorf("rendered output does not contain expected content")
			}
		})
	}
}
