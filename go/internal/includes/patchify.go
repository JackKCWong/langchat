// Package includes implements the {{ include "..." }} and {{ patchify ... }}
// directives used inside chat files. Includes resolve text or images
// relative to the chat file's directory (sandboxed unless AllowEscape is
// set), with a depth limit and cycle detection. Patchify splits an image
// into an m × n grid of overlapping tiles using anthonynsimon/bild.
package includes

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"path"
	"regexp"
	"strconv"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
)

// PatchifyDirectiveRE matches the canonical and whitespace-tolerant forms
// of the patchify directive:
//   {{ patchify "Goku.png", 2, 2, 30, 30 }}
var PatchifyDirectiveRE = regexp.MustCompile(
	`\{\{\s*patchify\s+"([^"]+)"\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*\}\}`,
)

// DefaultTileMIMEType matches the JS implementation's choice.
const DefaultTileMIMEType = "image/png"

// PatchifyArgs are the parsed arguments of a single patchify directive.
type PatchifyArgs struct {
	Path string
	M    int
	N    int
	X    float64
	Y    float64
}

// ParsePatchifyArgs extracts m, n, x, y from a directive match.
func ParsePatchifyArgs(match string) (PatchifyArgs, error) {
	m := PatchifyDirectiveRE.FindStringSubmatch(match)
	if m == nil {
		return PatchifyArgs{}, fmt.Errorf("patchify: cannot parse arguments from %q", match)
	}
	mn, _ := strconv.Atoi(m[2])
	nn, _ := strconv.Atoi(m[3])
	x, _ := strconv.ParseFloat(m[4], 64)
	y, _ := strconv.ParseFloat(m[5], 64)
	return PatchifyArgs{Path: m[1], M: mn, N: nn, X: x, Y: y}, nil
}

// ValidatePatchifyArgs enforces m, n >= 1 (integers) and x, y in [0, 100).
func ValidatePatchifyArgs(a PatchifyArgs) error {
	if a.M < 1 || float64(a.M) != math.Floor(float64(a.M)) {
		return fmt.Errorf("patchify: m must be a positive integer, got %d", a.M)
	}
	if a.N < 1 || float64(a.N) != math.Floor(float64(a.N)) {
		return fmt.Errorf("patchify: n must be a positive integer, got %d", a.N)
	}
	if a.X < 0 || a.X >= 100 || math.IsNaN(a.X) {
		return fmt.Errorf("patchify: x (vertical overlap %%) must be in [0, 100), got %v", a.X)
	}
	if a.Y < 0 || a.Y >= 100 || math.IsNaN(a.Y) {
		return fmt.Errorf("patchify: y (horizontal overlap %%) must be in [0, 100), got %v", a.Y)
	}
	return nil
}

// Tile is a single patch produced by Patchify.
type Tile struct {
	Row      int
	Col      int
	Left     int
	Top      int
	Width    int
	Height   int
	Patch    []byte
	MIMEType string
}

// ComputeGrid is exported for testing; it returns the m*n tile rects for a
// given source image size and overlap. Matches the JS implementation
// exactly (including the snap-to-edge behavior on the last row/column).
func ComputeGrid(W, H, m, n int, xPct, yPct float64) []TileRect {
	var tileH, strideH float64
	if m == 1 {
		tileH = float64(H)
		strideH = float64(H)
	} else {
		tileH = float64(H) / (1 + float64(m-1)*(1-xPct/100))
		strideH = tileH * (1 - xPct/100)
	}
	var tileW, strideW float64
	if n == 1 {
		tileW = float64(W)
		strideW = float64(W)
	} else {
		tileW = float64(W) / (1 + float64(n-1)*(1-yPct/100))
		strideW = tileW * (1 - yPct/100)
	}

	var out []TileRect
	for r := 0; r < m; r++ {
		for c := 0; c < n; c++ {
			top := math.Round(float64(r) * strideH)
			left := math.Round(float64(c) * strideW)
			if r == m-1 && m > 1 {
				top = math.Max(0, math.Round(float64(H)-tileH))
			}
			if c == n-1 && n > 1 {
				left = math.Max(0, math.Round(float64(W)-tileW))
			}
			out = append(out, TileRect{
				Row:    r,
				Col:    c,
				Left:   int(left),
				Top:    int(top),
				Width:  int(math.Round(tileW)),
				Height: int(math.Round(tileH)),
			})
		}
	}
	return out
}

// TileRect mirrors the JS tile descriptor shape (row, col, left, top, width, height).
type TileRect struct {
	Row           int
	Col           int
	Left, Top     int
	Width, Height int
}

// Patchify reads an image from raw bytes and splits it according to args.
// Tiles are re-encoded as PNG (matching JS, which uses sharp().png().toBuffer()).
func Patchify(buf []byte, args PatchifyArgs) ([]Tile, error) {
	if err := ValidatePatchifyArgs(args); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("patchify: cannot decode image: %v", err)
	}
	bounds := img.Bounds()
	W := bounds.Dx()
	H := bounds.Dy()
	if W == 0 || H == 0 {
		return nil, fmt.Errorf("patchify: cannot read image dimensions (got %dx%d)", W, H)
	}

	rects := ComputeGrid(W, H, args.M, args.N, args.X, args.Y)

	var tiles []Tile
	for _, rect := range rects {
		// Clamp to image bounds (matches the JS extractLeft/Top/Width/Height logic).
		left := rect.Left
		top := rect.Top
		if left < 0 {
			left = 0
		}
		if top < 0 {
			top = 0
		}
		if left > W-1 {
			left = W - 1
		}
		if top > H-1 {
			top = H - 1
		}
		ew := rect.Width
		if left+ew > W {
			ew = W - left
		}
		if ew < 1 {
			ew = 1
		}
		eh := rect.Height
		if top+eh > H {
			eh = H - top
		}
		if eh < 1 {
			eh = 1
		}

		cropped := transform.Crop(img, image.Rect(left, top, left+ew, top+eh))
		var buf bytes.Buffer
		if err := png.Encode(&buf, cropped); err != nil {
			return nil, fmt.Errorf("patchify: cannot encode tile: %v", err)
		}
		tiles = append(tiles, Tile{
			Row:      rect.Row,
			Col:      rect.Col,
			Left:     left,
			Top:      top,
			Width:    ew,
			Height:   eh,
			Patch:    buf.Bytes(),
			MIMEType: DefaultTileMIMEType,
		})
	}
	return tiles, nil
}

// SourceLabelFor returns "<base>[r<row>c<col>].png" for a tile. The base
// is the source filename without extension. Mirrors src/patchify.js.
func SourceLabelFor(rawPath string, row, col int) string {
	name := path.Base(rawPath)
	if dot := bytesLastIndex(name, '.'); dot > 0 {
		name = name[:dot]
	}
	return fmt.Sprintf("%s[r%dc%d].png", name, row, col)
}

func bytesLastIndex(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// encodeImage is a small wrapper used by the includes path when it needs to
// re-encode an image attachment that came from bild.
func encodeImage(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var _ = imgio.Open // keep imgio imported for future use (e.g. encoding formats)