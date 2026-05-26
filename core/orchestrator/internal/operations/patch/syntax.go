// Package patch — syntax pre-validation. Catches obvious LLM hallucinations
// (broken Go syntax, malformed JSON / YAML, unbalanced braces or quotes in
// TS/JS/Python) BEFORE a patch is allowed to mutate the project tree. This
// is the cheap, deterministic "first line of defense" that runs in-process
// in milliseconds — no workspace, no tooling installs required.
//
// Anything we can't parse natively in Go (TS/JS/Python ASTs) gets a quick
// delimiter-balance + obvious-truncation check. The full type/lint pass for
// those languages runs later inside the workspace via the Lint/Code gates.
package patch

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

// syntaxIssues returns one issue per file whose content fails the
// language-specific check. Empty content and unsupported extensions are
// silently skipped — emptiness is already flagged as a warning by the
// generic validator above.
func syntaxIssues(changes []FileChange) []domain.Issue {
	var out []domain.Issue
	for _, c := range changes {
		if c.Op == OpDelete || c.Content == "" {
			continue
		}
		if iss, ok := checkSyntax(c.Path, c.Content); ok {
			out = append(out, iss)
		}
	}
	return out
}

// checkSyntax dispatches the right parser per file extension. Returns
// (issue, true) on a real syntax error; (zero, false) when the file is
// either fine or in a language we don't pre-validate.
func checkSyntax(filePath, content string) (domain.Issue, bool) {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".go":
		return checkGo(filePath, content)
	case ".json":
		return checkJSON(filePath, content)
	case ".yaml", ".yml":
		return checkYAML(filePath, content)
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".py", ".rs", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".h", ".hpp":
		return checkDelimiters(filePath, content, ext)
	}
	return domain.Issue{}, false
}

// checkGo runs the standard library Go parser. Tolerant of partial input —
// we use parser.AllErrors so the first diagnostic is returned even when
// later lines also fail. Bonus: catches missing package declarations and
// malformed import blocks that a bracket-balance check would miss.
func checkGo(filePath, content string) (domain.Issue, bool) {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filePath, content, parser.AllErrors|parser.ParseComments)
	if err == nil {
		return domain.Issue{}, false
	}
	return domain.Issue{
		Gate:     domain.GateCode,
		Severity: domain.SeverityError,
		Message:  "Go syntax error: " + truncate(err.Error(), 240),
		Path:     filePath,
		Hint:     "fix the syntax before re-proposing the patch — the file would not compile",
	}, true
}

// checkJSON requires strict JSON. We refuse partial objects, trailing
// commas, comments — the same bar a real consumer enforces.
func checkJSON(filePath, content string) (domain.Issue, bool) {
	var v any
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return domain.Issue{
			Gate:     domain.GateCode,
			Severity: domain.SeverityError,
			Message:  "JSON parse error: " + truncate(err.Error(), 240),
			Path:     filePath,
			Hint:     "JSON must be strictly valid — no trailing commas, no comments",
		}, true
	}
	return domain.Issue{}, false
}

// checkYAML parses with yaml.v3 (already a transitive dep). Refuses files
// with unterminated quotes or duplicate map keys.
func checkYAML(filePath, content string) (domain.Issue, bool) {
	var v any
	if err := yaml.Unmarshal([]byte(content), &v); err != nil {
		return domain.Issue{
			Gate:     domain.GateCode,
			Severity: domain.SeverityError,
			Message:  "YAML parse error: " + truncate(err.Error(), 240),
			Path:     filePath,
			Hint:     "fix indentation or quoting — yaml.v3 strict mode is the bar",
		}, true
	}
	return domain.Issue{}, false
}

// checkDelimiters does a fast, deterministic balance pass on (), [], {}
// and quotes for languages we don't AST-parse in-process. It runs as a
// state machine so it correctly skips delimiters inside strings, char
// literals, and line/block comments per the comment style of the source
// extension. False positives are possible on exotic syntax (e.g. JSX
// inside JS strings) — when in doubt we lean to "no issue" to avoid
// blocking the loop on a parser disagreement.
func checkDelimiters(filePath, content, ext string) (domain.Issue, bool) {
	style := commentStyleFor(ext)
	var (
		paren, bracket, brace int
		inSingle, inDouble    bool
		inBacktick            bool
		inLineComment         bool
		inBlockComment        bool
		// Python triple-quote handling for """ and '''
		inTripleDouble bool
		inTripleSingle bool
		prev           rune
	)
	runes := []rune(content)
	at := func(i int) rune {
		if i < 0 || i >= len(runes) {
			return 0
		}
		return runes[i]
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		// Triple-quoted strings (Python) — handle BEFORE single-char string
		// state changes so """ doesn't double-flip inDouble.
		if style.python && !inLineComment && !inBlockComment && !inSingle && !inBacktick {
			if !inTripleSingle && r == '"' && at(i+1) == '"' && at(i+2) == '"' {
				inTripleDouble = !inTripleDouble
				i += 2
				prev = r
				continue
			}
			if !inTripleDouble && r == '\'' && at(i+1) == '\'' && at(i+2) == '\'' {
				inTripleSingle = !inTripleSingle
				i += 2
				prev = r
				continue
			}
			if inTripleDouble || inTripleSingle {
				prev = r
				continue
			}
		}
		if inLineComment {
			if r == '\n' {
				inLineComment = false
			}
			prev = r
			continue
		}
		if inBlockComment {
			if r == '/' && prev == '*' {
				inBlockComment = false
			}
			prev = r
			continue
		}
		if inSingle {
			if r == '\\' { // skip next escaped char
				i++
				prev = 0
				continue
			}
			if r == '\'' {
				inSingle = false
			}
			prev = r
			continue
		}
		if inDouble {
			if r == '\\' {
				i++
				prev = 0
				continue
			}
			if r == '"' {
				inDouble = false
			}
			prev = r
			continue
		}
		if inBacktick {
			if r == '`' {
				inBacktick = false
			}
			prev = r
			continue
		}
		// Open comment?
		if style.line != 0 && r == style.line && prev == style.line {
			inLineComment = true
			// step back one paren-count if we incremented on the prev char — n/a
			prev = r
			continue
		}
		if style.python && r == '#' {
			inLineComment = true
			prev = r
			continue
		}
		if style.block && r == '*' && prev == '/' {
			inBlockComment = true
			prev = r
			continue
		}
		switch r {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			if style.backtickString {
				inBacktick = true
			}
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		case '{':
			brace++
		case '}':
			brace--
		}
		if paren < 0 || bracket < 0 || brace < 0 {
			return domain.Issue{
				Gate:     domain.GateCode,
				Severity: domain.SeverityError,
				Message:  "unbalanced delimiter — extra closing bracket",
				Path:     filePath,
				Hint:     "the file has a stray ) ] or } before its opener",
			}, true
		}
		prev = r
	}
	if inDouble || inSingle || inBacktick || inTripleDouble || inTripleSingle {
		return domain.Issue{
			Gate:     domain.GateCode,
			Severity: domain.SeverityError,
			Message:  "unterminated string literal",
			Path:     filePath,
			Hint:     "make sure every opening quote has a matching close",
		}, true
	}
	if inBlockComment {
		return domain.Issue{
			Gate:     domain.GateCode,
			Severity: domain.SeverityError,
			Message:  "unterminated block comment",
			Path:     filePath,
			Hint:     "close the /* comment with */",
		}, true
	}
	if paren != 0 || bracket != 0 || brace != 0 {
		return domain.Issue{
			Gate:     domain.GateCode,
			Severity: domain.SeverityError,
			Message:  unbalancedMessage(paren, bracket, brace),
			Path:     filePath,
			Hint:     "balance the (), [], {} pairs before re-proposing",
		}, true
	}
	return domain.Issue{}, false
}

type commentStyle struct {
	line           rune // single character that, repeated twice, opens a line comment (e.g. '/')
	block          bool // /* … */ comments allowed
	python         bool // # line comments + triple-quoted strings
	backtickString bool // backtick template strings (JS/TS)
}

func commentStyleFor(ext string) commentStyle {
	switch ext {
	case ".py":
		return commentStyle{python: true}
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return commentStyle{line: '/', block: true, backtickString: true}
	case ".rs", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".h", ".hpp":
		return commentStyle{line: '/', block: true}
	}
	return commentStyle{}
}

func unbalancedMessage(paren, bracket, brace int) string {
	var parts []string
	if paren != 0 {
		parts = append(parts, "paren delta="+itoaSigned(paren))
	}
	if bracket != 0 {
		parts = append(parts, "bracket delta="+itoaSigned(bracket))
	}
	if brace != 0 {
		parts = append(parts, "brace delta="+itoaSigned(brace))
	}
	return "unbalanced delimiters: " + strings.Join(parts, ", ")
}

func itoaSigned(n int) string {
	if n >= 0 {
		return "+" + itoa(n)
	}
	return "-" + itoa(-n)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
