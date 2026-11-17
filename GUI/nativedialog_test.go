package main

import (
	"reflect"
	"testing"
)

func TestOpenSteps(t *testing.T) {
	tests := []struct {
		name string
		dir string
		want [][]string
	}{
		{"empty", "", [][]string{{"key", "--clearmodifiers", "ctrl+l"}}},
		{"directory", "/docs", [][]string{
			{"key", "--clearmodifiers", "ctrl+l"},
			{"type", "--clearmodifiers", "--delay", "1", "--", "/docs/"},
			{"key", "--clearmodifiers", "ctrl+a"},
		}},
		{"trailing slash", "/docs///", [][]string{
			{"key", "--clearmodifiers", "ctrl+l"},
			{"type", "--clearmodifiers", "--delay", "1", "--", "/docs/"},
			{"key", "--clearmodifiers", "ctrl+a"},
		}},
		{"root", "/", [][]string{
			{"key", "--clearmodifiers", "ctrl+l"},
			{"type", "--clearmodifiers", "--delay", "1", "--", "/"},
			{"key", "--clearmodifiers", "ctrl+a"},
		}},
		{"non-ASCII dir: Ctrl+L only", "/home/naïve", [][]string{{"key", "--clearmodifiers", "ctrl+l"}}},
		{"spaces and leading dash", "-my docs", [][]string{
			{"key", "--clearmodifiers", "ctrl+l"},
			{"type", "--clearmodifiers", "--delay", "1", "--", "-my docs/"},
			{"key", "--clearmodifiers", "ctrl+a"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := openSteps(tt.dir); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("openSteps(%q) = %#v, want %#v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestSaveSteps(t *testing.T) {
	tests := []struct {
		name string
		dir string
		filename string
		path string
		selection []string
	}{
		{"empty directory", "", "a.cdf", "", nil},
		{"both empty", "", "", "", nil},
		{"empty filename", "/docs", "", "/docs/", nil},
		{"trailing slash", "/docs///", "a.cdf", "/docs/a.cdf",
			[]string{"End", "Left", "Left", "Left", "Left", "shift+Left"}},
		{"root", "/", "a.cdf", "/a.cdf",
			[]string{"End", "Left", "Left", "Left", "Left", "shift+Left"}},
		{"multi dot", "/docs", "a.tar.gz", "/docs/a.tar.gz",
			[]string{"End", "Left", "Left", "Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left"}},
		{"dotfile", "/docs", ".bashrc", "/docs/.bashrc",
			[]string{"End", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left"}},
		{"no extension", "/docs", "README", "/docs/README",
			[]string{"End", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left", "shift+Left"}},
		{"non-ASCII stem skipped", "/docs", "naïve.cdf", "", nil},
		{"non-ASCII extension skipped", "/docs", "a.文", "", nil},
		{"non-ASCII dir skipped", "/Théologie", "a.cdf", "", nil},
		{"spaces", "/my docs", "a b.cdf", "/my docs/a b.cdf",
			[]string{"End", "Left", "Left", "Left", "Left", "shift+Left", "shift+Left", "shift+Left"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want [][]string
			if tt.path != "" {
				want = [][]string{
					{"key", "--clearmodifiers", "ctrl+a"},
					{"type", "--clearmodifiers", "--delay", "1", "--", tt.path},
				}
				if tt.selection != nil {
					want = append(want, append([]string{"key", "--clearmodifiers", "--delay", "1"}, tt.selection...))
				}
			}
			if got := saveSteps(tt.dir, tt.filename); !reflect.DeepEqual(got, want) {
				t.Fatalf("saveSteps(%q, %q) = %#v, want %#v", tt.dir, tt.filename, got, want)
			}
		})
	}
}

func TestSplitStem(t *testing.T) {
	tests := []struct {
		name string
		stem string
		ext string
	}{
		{"", "", ""},
		{"a.cdf", "a", ".cdf"},
		{"a.tar.gz", "a.tar", ".gz"},
		{".bashrc", ".bashrc", ""},
		{"README", "README", ""},
		{"naïve.cdf", "naïve", ".cdf"},
		{"a.文", "a", ".文"},
		{".config.json", ".config", ".json"},
		{"a.", "a", "."},
		{".", ".", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stem, ext := splitStem(tt.name)
			if stem != tt.stem || ext != tt.ext {
				t.Fatalf("splitStem(%q) = (%q, %q), want (%q, %q)", tt.name, stem, ext, tt.stem, tt.ext)
			}
		})
	}
}

func TestIsASCII(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"/home/user/my docs-1.cdf", true},
		{"~!@#$%^&*()_+{}|:<>?", true},
		{"naïve", false},
		{"/Théologie", false},
		{"文", false},
		{"\x7f", true},
	}
	for _, tt := range tests {
		if got := isASCII(tt.s); got != tt.want {
			t.Fatalf("isASCII(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestWithSlash(t *testing.T) {
	tests := []struct {
		dir string
		want string
	}{
		{"", "/"},
		{"/", "/"},
		{"///", "/"},
		{"/docs", "/docs/"},
		{"/docs/", "/docs/"},
		{"/docs///", "/docs/"},
		{"my docs", "my docs/"},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			if got := withSlash(tt.dir); got != tt.want {
				t.Fatalf("withSlash(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestParseWindowIDs(t *testing.T) {
	tests := []struct {
		name string
		out string
		want []string
	}{
		{"empty", "", nil},
		{"blank", " \n\t\r\n", nil},
		{"one", "123\n", []string{"123"}},
		{"many and whitespace", " 123 \n\n\t456\r\n 789\t\n", []string{"123", "456", "789"}},
		{"invalid", "0x123\n-1\n+1\n１２\n12a\n1.2\n12 34\n1\t2\n", nil},
		{"mixed", "bad\n123\n456x\n789\n", []string{"123", "789"}},
		{"decimal strings", "0\n00012\n999999999999999999999999\n", []string{"0", "00012", "999999999999999999999999"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseWindowIDs(tt.out); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseWindowIDs(%q) = %#v, want %#v", tt.out, got, tt.want)
			}
		})
	}
}
