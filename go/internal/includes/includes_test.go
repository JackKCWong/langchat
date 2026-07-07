package includes

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mkTmp(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "langchat-includes-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func write(t *testing.T, dir, name string, contents []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, contents, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{0xff, 0, 0, 0xff}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestSimpleInclude(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`before {{ include "snippet.txt" }} after`))
	write(t, dir, "snippet.txt", []byte("HELLO"))
	res, err := Resolve("before {{ include \"snippet.txt\" }} after", Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "before HELLO after" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestNestedInclude(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`{{ include "level1.md" }}`))
	write(t, dir, "level1.md", []byte(`L1 {{ include "leaf.txt" }} L1`))
	write(t, dir, "leaf.txt", []byte("LEAF"))
	res, err := Resolve(`{{ include "level1.md" }}`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "L1 LEAF L1" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestDiamondInclude(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`{{ include "a.md" }} || {{ include "b.md" }}`))
	write(t, dir, "a.md", []byte(`[A:{{ include "shared.txt" }}]`))
	write(t, dir, "b.md", []byte(`[B:{{ include "shared.txt" }}]`))
	write(t, dir, "shared.txt", []byte("S"))
	res, err := Resolve(`{{ include "a.md" }} || {{ include "b.md" }}`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "[A:S] || [B:S]" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestCyclicInclude(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "a.md", []byte(`{{ include "b.md" }}`))
	write(t, dir, "b.md", []byte(`{{ include "a.md" }}`))
	_, err := Resolve(`{{ include "a.md" }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "cyclic include detected") {
		t.Errorf("err = %v", err)
	}
}

func TestMissingInclude(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`{{ include "nope.txt" }}`))
	_, err := Resolve(`{{ include "nope.txt" }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "include failed: nope.txt") {
		t.Errorf("err = %v", err)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	dir := mkTmp(t)
	outside := mkTmp(t)
	write(t, outside, "secret.txt", []byte("SECRET"))
	write(t, dir, "chat.md", []byte(`{{ include "../secret.txt" }}`))
	_, err := Resolve(`{{ include "../secret.txt" }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "escapes base directory") {
		t.Errorf("err = %v", err)
	}
}

func TestAllowEscape(t *testing.T) {
	parent := mkTmp(t)
	inner := filepath.Join(parent, "inner")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, outside, "secret.txt", []byte("SECRET"))
	write(t, inner, "chat.md", []byte(`{{ include "../outside/secret.txt" }}`))
	res, err := Resolve(`{{ include "../outside/secret.txt" }}`, Options{BaseDir: inner, AllowEscape: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "SECRET" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestAbsolutePath(t *testing.T) {
	dir := mkTmp(t)
	target := filepath.Join(dir, "abs.txt")
	if err := os.WriteFile(target, []byte("ABS"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := Resolve(`{{ include "`+target+`" }}`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "ABS" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestMaxDepth(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "leaf.txt", []byte("BOTTOM"))
	prev := "leaf.txt"
	for i := 0; i < 10; i++ {
		next := "level" + intToStr(i) + ".md"
		write(t, dir, next, []byte(`{{ include "`+prev+`" }}`))
		prev = next
	}
	_, err := Resolve(`{{ include "`+prev+`" }}`, Options{BaseDir: dir, MaxDepth: 3})
	if err == nil || !strings.Contains(err.Error(), "include depth exceeded 3") {
		t.Errorf("err = %v", err)
	}
}

func TestLooseWhitespace(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "a.txt", []byte("A"))
	write(t, dir, "b.txt", []byte("B"))
	write(t, dir, "c.txt", []byte("C"))
	md := "{{include \"a.txt\"}}\n{{  include  \"b.txt\"  }}\n{{\ninclude \"c.txt\"\n}}"
	res, err := Resolve(md, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "A\nB\nC" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestLeavesTextWithoutDirectivesUnchanged(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "x", []byte("X"))
	md := "plain text {{ not include }} {{ include }} {{include \"x\"}}"
	res, err := Resolve(md, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "plain text {{ not include }} {{ include }} X" {
		t.Errorf("text = %q", res.Text)
	}
}

func TestExpansionIntegratesWithChatParser(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`# !user

Answer based on: {{ include "ctx.txt" }}
`))
	write(t, dir, "ctx.txt", []byte("the context"))
	res, err := Resolve(`# !user

Answer based on: {{ include "ctx.txt" }}
`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// (We don't call the parser here; this test just checks resolve produced
	// the expected expansion so the parser can be tested separately.)
	if !strings.Contains(res.Text, "the context") {
		t.Errorf("text = %q", res.Text)
	}
}

func TestImageIncludeLeavesDirectiveAndRecordsAttachment(t *testing.T) {
	dir := mkTmp(t)
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	write(t, dir, "pic.png", pngBytes)
	write(t, dir, "chat.md", []byte(`see {{ include "pic.png" }} here`))
	res, err := Resolve(`see {{ include "pic.png" }} here`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != `see {{ include "pic.png" }} here` {
		t.Errorf("text = %q", res.Text)
	}
	if len(res.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	a := res.Attachments[0]
	if a.Type != "image" || a.MIMEType != "image/png" || a.Source != "pic.png" {
		t.Errorf("attachment = %+v", a)
	}
	// base64 of the pngBytes above
	if a.Data != "iVBORw0KGgo=" {
		t.Errorf("data = %q", a.Data)
	}
}

func TestMapsEverySupportedExtensionToMime(t *testing.T) {
	dir := mkTmp(t)
	fixtures := []struct {
		name string
		mime string
	}{
		{"a.png", "image/png"},
		{"b.jpg", "image/jpeg"},
		{"c.jpeg", "image/jpeg"},
		{"d.gif", "image/gif"},
		{"e.webp", "image/webp"},
	}
	var md strings.Builder
	for i, f := range fixtures {
		if i > 0 {
			md.WriteString("|")
		}
		md.WriteString(`{{ include "` + f.name + `" }}`)
		write(t, dir, f.name, []byte{0})
	}
	res, err := Resolve(md.String(), Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Attachments) != len(fixtures) {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	for i, f := range fixtures {
		if res.Attachments[i].MIMEType != f.mime {
			t.Errorf("attachment[%d] mime = %q, want %q", i, res.Attachments[i].MIMEType, f.mime)
		}
	}
}

func TestAttachmentsInLeftToRightOrder(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "a.png", []byte{1})
	write(t, dir, "b.txt", []byte("TXT"))
	write(t, dir, "c.jpg", []byte{2})
	write(t, dir, "chat.md", []byte(`1:{{ include "a.png" }} 2:{{ include "b.txt" }} 3:{{ include "c.jpg" }}`))
	res, err := Resolve(`1:{{ include "a.png" }} 2:{{ include "b.txt" }} 3:{{ include "c.jpg" }}`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != `1:{{ include "a.png" }} 2:TXT 3:{{ include "c.jpg" }}` {
		t.Errorf("text = %q", res.Text)
	}
	if len(res.Attachments) != 2 {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	if res.Attachments[0].MIMEType != "image/png" || res.Attachments[1].MIMEType != "image/jpeg" {
		t.Errorf("attachments = %+v", res.Attachments)
	}
}

func TestUnknownExtensionsAreText(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "doc.bmp", []byte("NOT-AN-IMAGE-BUT-STILL-TEXT"))
	res, err := Resolve(`{{ include "doc.bmp" }}`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != "NOT-AN-IMAGE-BUT-STILL-TEXT" {
		t.Errorf("text = %q", res.Text)
	}
	if len(res.Attachments) != 0 {
		t.Errorf("attachments = %d", len(res.Attachments))
	}
}

func TestSizeCapOnImage(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "big.png", make([]byte, 64))
	_, err := Resolve(`{{ include "big.png" }}`, Options{BaseDir: dir, MaxBytes: 16})
	if err == nil || !strings.Contains(err.Error(), "limit is 16 bytes") {
		t.Errorf("err = %v", err)
	}
}

func TestSizeCapOnText(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "big.txt", bytes.Repeat([]byte{'x'}, 1024))
	_, err := Resolve(`{{ include "big.txt" }}`, Options{BaseDir: dir, MaxBytes: 64})
	if err == nil || !strings.Contains(err.Error(), "exceeds 64 bytes") {
		t.Errorf("err = %v", err)
	}
}

func TestImageIncludeRespectsCycle(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "pic.png", []byte{0})
	write(t, dir, "chat.md", []byte(`{{ include "chat.md" }}`))
	_, err := Resolve(`{{ include "chat.md" }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "cyclic include detected") {
		t.Errorf("err = %v", err)
	}
}

func TestImageIncludeRespectsPathTraversal(t *testing.T) {
	parent := mkTmp(t)
	inner := filepath.Join(parent, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write(t, parent, "pic.png", []byte{0})
	write(t, inner, "chat.md", []byte(`{{ include "../pic.png" }}`))
	_, err := Resolve(`{{ include "../pic.png" }}`, Options{BaseDir: inner})
	if err == nil || !strings.Contains(err.Error(), "escapes base directory") {
		t.Errorf("err = %v", err)
	}
}

func TestImageIncludeRespectsMaxDepth(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "leaf.png", []byte{0})
	prev := "leaf.png"
	for i := 0; i < 10; i++ {
		next := "level" + intToStr(i) + ".md"
		write(t, dir, next, []byte(`{{ include "`+prev+`" }}`))
		prev = next
	}
	_, err := Resolve(`{{ include "`+prev+`" }}`, Options{BaseDir: dir, MaxDepth: 3})
	if err == nil || !strings.Contains(err.Error(), "include depth exceeded 3") {
		t.Errorf("err = %v", err)
	}
}

func TestNestedTextFileIncludesImage(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "inner.png", []byte{7, 7, 7})
	write(t, dir, "helper.md", []byte(`see {{ include "inner.png" }} end`))
	write(t, dir, "chat.md", []byte(`before {{ include "helper.md" }} after`))
	res, err := Resolve(`before {{ include "helper.md" }} after`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Text != `before see {{ include "inner.png" }} end after` {
		t.Errorf("text = %q", res.Text)
	}
	if len(res.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	a := res.Attachments[0]
	if a.MIMEType != "image/png" || a.Source != "inner.png" {
		t.Errorf("attachment = %+v", a)
	}
}

func TestResolveIncludesExpandsPatchifyDirective(t *testing.T) {
	dir := mkTmp(t)
	imgPath := filepath.Join(dir, "a.png")
	os.WriteFile(imgPath, makePNG(t, 100, 80), 0o644)
	write(t, dir, "chat.md", []byte(`see {{ patchify "a.png", 2, 2, 0, 0 }} here`))
	res, err := Resolve(`see {{ patchify "a.png", 2, 2, 0, 0 }} here`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := `see {{ include "a[r0c0].png" }} {{ include "a[r0c1].png" }} {{ include "a[r1c0].png" }} {{ include "a[r1c1].png" }} here`
	if res.Text != want {
		t.Errorf("text = %q, want %q", res.Text, want)
	}
	if len(res.Attachments) != 4 {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	for _, a := range res.Attachments {
		if a.Type != "image" || a.MIMEType != "image/png" {
			t.Errorf("attachment = %+v", a)
		}
	}
	wantSources := []string{"a[r0c0].png", "a[r0c1].png", "a[r1c0].png", "a[r1c1].png"}
	var gotSources []string
	for _, a := range res.Attachments {
		gotSources = append(gotSources, a.Source)
	}
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Errorf("sources = %v, want %v", gotSources, wantSources)
	}
}

func TestResolveIncludesPreservesOrderWithPatchify(t *testing.T) {
	dir := mkTmp(t)
	os.WriteFile(filepath.Join(dir, "a.png"), makePNG(t, 100, 80), 0o644)
	os.WriteFile(filepath.Join(dir, "b.jpg"), []byte{0xff, 0xd8, 0xff, 0xd8}, 0o644)
	write(t, dir, "chat.md", []byte(`1:{{ include "b.jpg" }} 2:{{ patchify "a.png", 2, 2, 0, 0 }} 3:end`))
	res, err := Resolve(`1:{{ include "b.jpg" }} 2:{{ patchify "a.png", 2, 2, 0, 0 }} 3:end`, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Attachments) != 5 {
		t.Fatalf("attachments = %d", len(res.Attachments))
	}
	wantSources := []string{"b.jpg", "a[r0c0].png", "a[r0c1].png", "a[r1c0].png", "a[r1c1].png"}
	for i, w := range wantSources {
		if res.Attachments[i].Source != w {
			t.Errorf("attachments[%d].Source = %q, want %q", i, res.Attachments[i].Source, w)
		}
	}
	if !strings.Contains(res.Text, `1:{{ include "b.jpg" }}`) {
		t.Errorf("text missing b.jpg ref: %q", res.Text)
	}
	if !strings.Contains(res.Text, `2:{{ include "a[r0c0].png" }}`) {
		t.Errorf("text missing r0c0 ref: %q", res.Text)
	}
}

func TestResolveIncludesBlocksPathTraversalOnPatchify(t *testing.T) {
	dir := mkTmp(t)
	outside := mkTmp(t)
	os.WriteFile(filepath.Join(outside, "secret.png"), makePNG(t, 40, 40), 0o644)
	write(t, dir, "chat.md", []byte(`{{ patchify "../secret.png", 2, 2, 0, 0 }}`))
	_, err := Resolve(`{{ patchify "../secret.png", 2, 2, 0, 0 }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "escapes base directory") {
		t.Errorf("err = %v", err)
	}
}

func TestResolveIncludesRejectsInvalidPatchifyArgs(t *testing.T) {
	dir := mkTmp(t)
	os.WriteFile(filepath.Join(dir, "a.png"), makePNG(t, 100, 80), 0o644)
	write(t, dir, "chat.md", []byte(`{{ patchify "a.png", 0, 2, 0, 0 }}`))
	_, err := Resolve(`{{ patchify "a.png", 0, 2, 0, 0 }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "m must be a positive integer") {
		t.Errorf("err = %v", err)
	}
}

func TestResolveIncludesRejectsPatchifyXYTooLarge(t *testing.T) {
	dir := mkTmp(t)
	os.WriteFile(filepath.Join(dir, "a.png"), makePNG(t, 100, 80), 0o644)
	write(t, dir, "chat.md", []byte(`{{ patchify "a.png", 2, 2, 100, 0 }}`))
	_, err := Resolve(`{{ patchify "a.png", 2, 2, 100, 0 }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "x (vertical overlap") {
		t.Errorf("err = %v", err)
	}
}

func TestResolveIncludesRejectsMissingPatchifySource(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "chat.md", []byte(`{{ patchify "nope.png", 2, 2, 0, 0 }}`))
	_, err := Resolve(`{{ patchify "nope.png", 2, 2, 0, 0 }}`, Options{BaseDir: dir})
	if err == nil || !strings.Contains(err.Error(), "patchify failed: nope.png") {
		t.Errorf("err = %v", err)
	}
}

func TestResolveIncludesAppliesSizeCapToPatchify(t *testing.T) {
	dir := mkTmp(t)
	write(t, dir, "big.png", make([]byte, 64))
	write(t, dir, "chat.md", []byte(`{{ patchify "big.png", 2, 2, 0, 0 }}`))
	_, err := Resolve(`{{ patchify "big.png", 2, 2, 0, 0 }}`, Options{BaseDir: dir, MaxBytes: 16})
	if err == nil || !strings.Contains(err.Error(), "limit is 16 bytes") {
		t.Errorf("err = %v", err)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}