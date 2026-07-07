package includes

import "regexp"

// includeDirectiveRE matches `{{ include "..." }}` with optional whitespace.
var includeDirectiveRE = regexp.MustCompile(`\{\{\s*include\s+"([^"]+)"\s*\}\}`)

// includeMatch is a single regex hit: start/end are byte offsets into the
// input, path is the captured filename, full is the entire match.
type includeMatch struct {
	Start, End int
	Path, Full string
}

func includeMatches(s string) []includeMatch {
	out := []includeMatch{}
	for _, m := range includeDirectiveRE.FindAllStringSubmatchIndex(s, -1) {
		out = append(out, includeMatch{
			Start: m[0],
			End:   m[1],
			Path:  s[m[2]:m[3]],
			Full:  s[m[0]:m[1]],
		})
	}
	return out
}

// patchifyMatch is a single patchify hit: start/end are byte offsets, full
// is the entire match.
type patchifyMatch struct {
	Start, End int
	Full       string
}

func patchifyMatches(s string) []patchifyMatch {
	out := []patchifyMatch{}
	for _, m := range PatchifyDirectiveRE.FindAllStringSubmatchIndex(s, -1) {
		out = append(out, patchifyMatch{
			Start: m[0],
			End:   m[1],
			Full:  s[m[0]:m[1]],
		})
	}
	return out
}