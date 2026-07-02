package lamvms

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

var readmeFiles = []string{"README.md", "README.ja.md"}

var globalFlagsHeadings = []string{"## Global Flags", "## グローバルフラグ"}

var commandsHeadings = []string{"## Commands", "## コマンド"}

func TestREADMEDocumentsAllFlags(t *testing.T) {
	t.Parallel()
	var opts CLIOptions
	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range readmeFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			readme := string(data)

			globalSection := headingSection(readme, globalFlagsHeadings)
			if globalSection == "" {
				t.Fatalf("global flags section not found (looked for %v)", globalFlagsHeadings)
			}
			for _, f := range parser.Model.Flags {
				if f.Name == "help" || f.Hidden {
					continue
				}
				if !flagDocumented(globalSection, f) {
					t.Errorf("global flag --%s is not documented in the global flags section", f.Name)
				}
			}

			commandsRegion := headingSection(readme, commandsHeadings)
			if commandsRegion == "" {
				t.Fatalf("commands section not found (looked for %v)", commandsHeadings)
			}
			for _, cmd := range parser.Model.Children {
				if cmd.Hidden || len(collectFlags(cmd)) == 0 {
					continue
				}
				section := commandSection(commandsRegion, cmd.Name)
				if section == "" {
					t.Errorf("command %q has no section under Commands", cmd.Name)
					continue
				}
				for _, f := range collectFlags(cmd) {
					if f.Name == "help" || f.Hidden {
						continue
					}
					if !flagDocumented(section, f) {
						t.Errorf("command %q: flag --%s is not documented in its section", cmd.Name, f.Name)
					}
				}
			}
		})
	}
}

func collectFlags(node *kong.Node) []*kong.Flag {
	flags := append([]*kong.Flag{}, node.Flags...)
	for _, child := range node.Children {
		flags = append(flags, collectFlags(child)...)
	}
	return flags
}

// commandSection returns the body of the "### ..." section whose heading
// contains the command name as a whole word. Combined headings such as
// "### suspend / resume / terminate" match each of their command names.
func commandSection(readme, command string) string {
	re := regexp.MustCompile(`(?m)^### .*\b` + regexp.QuoteMeta(command) + `\b.*$`)
	loc := re.FindStringIndex(readme)
	if loc == nil {
		return ""
	}
	return sectionBody(readme[loc[1]:])
}

// headingSection returns the body of the first matching "## ..." heading,
// up to the next "## " heading (subsections included).
func headingSection(readme string, headings []string) string {
	for _, h := range headings {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(h) + `$`)
		loc := re.FindStringIndex(readme)
		if loc != nil {
			return cutAt(readme[loc[1]:], nextH2Re)
		}
	}
	return ""
}

var (
	nextH2Re      = regexp.MustCompile(`(?m)^## `)
	nextHeadingRe = regexp.MustCompile(`(?m)^#{2,3} `)
)

func sectionBody(rest string) string {
	return cutAt(rest, nextHeadingRe)
}

func cutAt(rest string, re *regexp.Regexp) string {
	if loc := re.FindStringIndex(rest); loc != nil {
		return rest[:loc[0]]
	}
	return rest
}

func flagDocumented(section string, f *kong.Flag) bool {
	if strings.Contains(section, "--"+f.Name) {
		return true
	}
	return f.Short != 0 && strings.Contains(section, "`-"+string(f.Short))
}
