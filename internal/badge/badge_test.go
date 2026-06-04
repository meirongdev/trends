package badge

import (
	"strings"
	"testing"
)

func TestRenderContainsLabelMessageAndSVG(t *testing.T) {
	svg := Render("trends", "rank #3", "#3aa3e3")
	for _, want := range []string{"<svg", "</svg>", "trends", "rank #3", "#3aa3e3", `height="20"`} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %q\n%s", want, svg)
		}
	}
}

func TestRenderEscapesXML(t *testing.T) {
	svg := Render("a&b", "x<y", "#555")
	if strings.Contains(svg, "a&b") || strings.Contains(svg, "x<y") {
		t.Fatalf("unescaped XML in svg: %s", svg)
	}
	if !strings.Contains(svg, "a&amp;b") || !strings.Contains(svg, "x&lt;y") {
		t.Fatalf("expected escaped entities: %s", svg)
	}
}
