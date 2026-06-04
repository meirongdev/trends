// Package badge 生成 shields.io 风格的扁平 SVG 徽章(左 label 灰、右 message 着色)。
package badge

import (
	"fmt"
	"strings"
)

const (
	charWidth = 7  // 每字符近似像素宽(ASCII 估算)
	padding   = 10 // 每段左右内边距合计
)

func textWidth(s string) int { return len([]rune(s))*charWidth + padding }

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// Render 返回一个宽度自适应的扁平徽章 SVG。label/message 会做 XML 转义;
// color 由调用方控制(十六进制颜色)。
func Render(label, message, color string) string {
	lw, mw := textWidth(label), textWidth(message)
	w := lw + mw
	l, m := xmlEscaper.Replace(label), xmlEscaper.Replace(message)
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">`+
			`<title>%s: %s</title>`+
			`<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>`+
			`<clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath>`+
			`<g clip-path="url(#r)">`+
			`<rect width="%d" height="20" fill="#555"/>`+
			`<rect x="%d" width="%d" height="20" fill="%s"/>`+
			`<rect width="%d" height="20" fill="url(#s)"/>`+
			`</g>`+
			`<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">`+
			`<text x="%d" y="15">%s</text>`+
			`<text x="%d" y="15">%s</text>`+
			`</g></svg>`,
		w, l, m, l, m, w, lw, lw, mw, color, w, lw/2, l, lw+mw/2, m)
}
