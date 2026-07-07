package includes

import (
	"bytes"
	"image"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestPatchifyDirectiveREMatchesCanonical(t *testing.T) {
	m := PatchifyDirectiveRE.FindStringSubmatch(`{{ patchify "Goku.png", 2, 2, 5, 10 }}`)
	if m == nil {
		t.Fatalf("no match")
	}
	want := []string{
		`{{ patchify "Goku.png", 2, 2, 5, 10 }}`,
		"Goku.png", "2", "2", "5", "10",
	}
	if !reflect.DeepEqual(m, want) {
		t.Errorf("m = %v, want %v", m, want)
	}
}

func TestPatchifyDirectiveREToleratesWhitespace(t *testing.T) {
	variants := []string{
		`{{patchify "a.png",2,3,0.5,10}}`,
		`{{  patchify  "b.png" , 1 , 4 , 99 , 0  }}`,
		"{{\npatchify \"c.png\",\n3,\n3,\n25,\n25\n}}",
	}
	for _, v := range variants {
		if !PatchifyDirectiveRE.MatchString(v) {
			t.Errorf("did not match: %q", v)
		}
	}
}

func TestParsePatchifyArgs(t *testing.T) {
	out, err := ParsePatchifyArgs(`{{ patchify "a.png", 2, 3, 25, 0.5 }}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := PatchifyArgs{Path: "a.png", M: 2, N: 3, X: 25, Y: 0.5}
	if out != want {
		t.Errorf("got %+v, want %+v", out, want)
	}
}

func TestParsePatchifyArgsMalformed(t *testing.T) {
	if _, err := ParsePatchifyArgs("not a directive"); err == nil {
		t.Errorf("expected error")
	} else if !strings.Contains(err.Error(), "cannot parse arguments") {
		t.Errorf("err = %v", err)
	}
}

func TestValidatePatchifyArgsMVP5Example(t *testing.T) {
	if err := ValidatePatchifyArgs(PatchifyArgs{M: 2, N: 2, X: 5, Y: 10}); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestValidatePatchifyArgsRejectsNonPositive(t *testing.T) {
	cases := []struct {
		args PatchifyArgs
		want string
	}{
		{PatchifyArgs{M: 0, N: 2, X: 0, Y: 0}, "m"},
		{PatchifyArgs{M: -1, N: 2, X: 0, Y: 0}, "m"},
		{PatchifyArgs{M: 2, N: 0, X: 0, Y: 0}, "n"},
	}
	for _, c := range cases {
		err := ValidatePatchifyArgs(c.args)
		if err == nil {
			t.Errorf("expected error for %+v", c.args)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("err %q does not contain %q", err.Error(), c.want)
		}
	}

	// Float-valued M is checked via the type system in Go; verify that a
	// value which would round-trip to non-integer M is still rejected. Since
	// the field is typed int we can't directly test "2.5"; instead verify
	// the integer-valued path accepts valid m.
	if err := ValidatePatchifyArgs(PatchifyArgs{M: 2, N: 2, X: 0, Y: 0}); err != nil {
		t.Errorf("m=2 should be valid: %v", err)
	}
}

func TestValidatePatchifyArgsRejectsXYOutOfRange(t *testing.T) {
	cases := []PatchifyArgs{
		{M: 2, N: 2, X: -1, Y: 0},
		{M: 2, N: 2, X: 0, Y: -5},
		{M: 2, N: 2, X: 100, Y: 0},
		{M: 2, N: 2, X: 0, Y: 100},
	}
	for _, c := range cases {
		if err := ValidatePatchifyArgs(c); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
}

func TestComputeGridRowMajorOrder(t *testing.T) {
	grid := ComputeGrid(200, 100, 2, 3, 0, 0)
	if len(grid) != 6 {
		t.Fatalf("grid len = %d, want 6", len(grid))
	}
	for r := 0; r < 2; r++ {
		for c := 0; c < 3; c++ {
			i := r*3 + c
			if grid[i].Row != r || grid[i].Col != c {
				t.Errorf("grid[%d] = %+v, want row=%d col=%d", i, grid[i], r, c)
			}
		}
	}
}

func TestComputeGridZeroOverlapTiling(t *testing.T) {
	grid := ComputeGrid(100, 80, 2, 2, 0, 0)
	if len(grid) != 4 {
		t.Fatalf("grid len = %d, want 4", len(grid))
	}
	for _, t1 := range grid {
		if t1.Width != 50 || t1.Height != 40 {
			t.Errorf("tile = %+v, want 50x40", t1)
		}
	}
	want := []TileRect{
		{Row: 0, Col: 0, Left: 0, Top: 0, Width: 50, Height: 40},
		{Row: 0, Col: 1, Left: 50, Top: 0, Width: 50, Height: 40},
		{Row: 1, Col: 0, Left: 0, Top: 40, Width: 50, Height: 40},
		{Row: 1, Col: 1, Left: 50, Top: 40, Width: 50, Height: 40},
	}
	if !reflect.DeepEqual(grid, want) {
		t.Errorf("grid = %+v", grid)
	}
}

func TestComputeGridVerticalOverlapBottomAlign(t *testing.T) {
	grid := ComputeGrid(100, 80, 2, 2, 50, 0)
	if grid[0].Top != 0 || grid[1].Top != 0 {
		t.Errorf("top rows should be 0, got %d %d", grid[0].Top, grid[1].Top)
	}
	if grid[2].Top+grid[2].Height != 80 || grid[3].Top+grid[3].Height != 80 {
		t.Errorf("bottom rows should reach H=80, got %d + %d = %d", grid[2].Top, grid[2].Height, grid[2].Top+grid[2].Height)
	}
}

func TestComputeGridHorizontalOverlapRightAlign(t *testing.T) {
	grid := ComputeGrid(100, 80, 2, 2, 0, 50)
	if grid[0].Left != 0 || grid[2].Left != 0 {
		t.Errorf("left cols should be 0, got %d %d", grid[0].Left, grid[2].Left)
	}
	for _, r := range []int{0, 1} {
		i := r*2 + 1
		if grid[i].Left+grid[i].Width != 100 {
			t.Errorf("right col row %d: left+width = %d, want 100", r, grid[i].Left+grid[i].Width)
		}
	}
}

func TestComputeGridM1FullHeight(t *testing.T) {
	grid := ComputeGrid(100, 80, 1, 4, 99, 0)
	for _, t1 := range grid {
		if t1.Height != 80 || t1.Top != 0 {
			t.Errorf("tile = %+v, want height=80 top=0", t1)
		}
	}
	if grid[0].Left != 0 {
		t.Errorf("first tile left = %d, want 0", grid[0].Left)
	}
	if grid[3].Left+grid[3].Width != 100 {
		t.Errorf("last tile left+width = %d, want 100", grid[3].Left+grid[3].Width)
	}
}

func TestComputeGridN1FullWidth(t *testing.T) {
	grid := ComputeGrid(100, 80, 4, 1, 0, 99)
	for _, t1 := range grid {
		if t1.Width != 100 || t1.Left != 0 {
			t.Errorf("tile = %+v, want width=100 left=0", t1)
		}
	}
}

func TestSourceLabelFor(t *testing.T) {
	cases := []struct {
		path       string
		row, col   int
		want       string
	}{
		{"Goku.png", 0, 0, "Goku[r0c0].png"},
		{"Goku.png", 1, 2, "Goku[r1c2].png"},
		{"nested/path/a.bmp", 0, 0, "a[r0c0].png"},
		{"noext", 2, 3, "noext[r2c3].png"},
	}
	for _, c := range cases {
		got := SourceLabelFor(c.path, c.row, c.col)
		if got != c.want {
			t.Errorf("SourceLabelFor(%q, %d, %d) = %q, want %q", c.path, c.row, c.col, got, c.want)
		}
	}
}

// makePNG lives in includes_test.go and is shared with patchify_test.go.

func TestPatchifyImage1x1CoversWhole(t *testing.T) {
	buf := makePNG(t, 40, 30)
	tiles, err := Patchify(buf, PatchifyArgs{M: 1, N: 1, X: 5, Y: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(tiles) != 1 {
		t.Fatalf("len = %d, want 1", len(tiles))
	}
	t1 := tiles[0]
	if t1.Row != 0 || t1.Col != 0 || t1.Width != 40 || t1.Height != 30 || t1.Left != 0 || t1.Top != 0 {
		t.Errorf("tile = %+v", t1)
	}
	if t1.MIMEType != DefaultTileMIMEType {
		t.Errorf("mime = %q, want %q", t1.MIMEType, DefaultTileMIMEType)
	}
	meta, _, err := image.Decode(bytes.NewReader(t1.Patch))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if meta.Bounds().Dx() != 40 || meta.Bounds().Dy() != 30 {
		t.Errorf("decoded = %dx%d, want 40x30", meta.Bounds().Dx(), meta.Bounds().Dy())
	}
}

func TestPatchifyImageMNReencodedAsPNG(t *testing.T) {
	buf := makePNG(t, 100, 80)
	tiles, err := Patchify(buf, PatchifyArgs{M: 2, N: 2, X: 5, Y: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(tiles) != 4 {
		t.Fatalf("len = %d, want 4", len(tiles))
	}
	for _, t1 := range tiles {
		if t1.MIMEType != "image/png" {
			t.Errorf("mime = %q", t1.MIMEType)
		}
		if _, _, err := image.Decode(bytes.NewReader(t1.Patch)); err != nil {
			t.Errorf("tile not decodable: %v", err)
		}
	}
}

func TestPatchifyImageZeroOverlapPartitionsExactly(t *testing.T) {
	buf := makePNG(t, 100, 80)
	tiles, err := Patchify(buf, PatchifyArgs{M: 2, N: 2, X: 0, Y: 0})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, t1 := range tiles {
		if t1.Width != 50 || t1.Height != 40 {
			t.Errorf("tile = %+v", t1)
		}
	}
	byKey := func(r, c int) Tile {
		for _, t1 := range tiles {
			if t1.Row == r && t1.Col == c {
				return t1
			}
		}
		t.Fatalf("missing %d,%d", r, c)
		return Tile{}
	}
	if byKey(0, 0).Left != 0 || byKey(0, 0).Top != 0 {
		t.Errorf("(0,0) = %+v", byKey(0, 0))
	}
	if byKey(0, 1).Left != 50 {
		t.Errorf("(0,1) Left = %d", byKey(0, 1).Left)
	}
	if byKey(1, 0).Top != 40 {
		t.Errorf("(1,0) Top = %d", byKey(1, 0).Top)
	}
	if byKey(1, 1).Left != 50 || byKey(1, 1).Top != 40 {
		t.Errorf("(1,1) = %+v", byKey(1, 1))
	}
}

func TestPatchifyRegexIsAccessible(t *testing.T) {
	// Just a sanity check that the regex variable is exported.
	m := PatchifyDirectiveRE.MatchString(`x {{ patchify "a.png", 1, 1, 0, 0 }} y`)
	if !m {
		t.Errorf("expected match")
	}
	if regexp.MustCompile(PatchifyDirectiveRE.String()).NumSubexp() != 5 {
		t.Errorf("expected 5 capture groups")
	}
}