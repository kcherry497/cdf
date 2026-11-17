package main

import (
	"fmt"
	"strings"
)

// ToHTML — same visual language as the GUI, but rendered as a self-contained
// HTML document for one-off sharing with someone who doesn't have `cdf`.
// Read-only; matches spec.md.

func htmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func ToHTML(c *CDF) string {
	var b strings.Builder
	b.WriteString(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>`)
	b.WriteString(htmlEsc(c.Meta.Title))
	b.WriteString(`</title>
<style>
:root{--bg:#0d0e11;--surface:#14161b;--border:#2a2e37;--ink:#e8ebef;--muted:#7a8090;--accent:#4fd1c5;--warn:#d4a94a;--danger:#e07070}
*{box-sizing:border-box}html,body{background:var(--bg)}
body{color:var(--ink);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Helvetica Neue",Arial,sans-serif;font-size:15px;line-height:1.55;margin:0}
.page{max-width:960px;margin:0 auto;padding:3rem 1.5rem 6rem}
h1{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-size:1.55rem;font-weight:700;margin:0 0 .3rem}
.sub{color:var(--muted);font-size:.9rem;margin-bottom:1.25rem;padding-bottom:1.25rem;border-bottom:1px solid var(--border)}
section{margin-top:2.75rem}
h2{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-size:1.1rem;color:var(--accent);margin:0 0 .4rem}
.note{color:var(--muted);font-size:.9rem;margin:0 0 .75rem;max-width:68ch}
.tw{overflow-x:auto}
table{width:100%;border-collapse:collapse;font-size:.88rem;font-variant-numeric:tabular-nums;border:1px solid var(--border)}
th{text-align:left;font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-weight:500;font-size:.66rem;letter-spacing:.09em;text-transform:uppercase;color:var(--muted);padding:.5rem .75rem;background:var(--surface);border:1px solid var(--border)}
td{padding:.72rem .75rem;border:1px solid var(--border);vertical-align:top}
.date{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;color:var(--muted);white-space:nowrap;font-size:.82rem;width:110px}
.action{font-weight:600;width:30%}
.marker{display:inline-block;width:3px;height:.9em;background:var(--muted);margin-right:.55rem;vertical-align:-1px;border-radius:1px}
tr.milestone .marker{background:var(--accent)}
tr.exposure .marker{background:var(--danger)}
tr.deferred .marker{background:var(--warn)}
</style></head><body><div class="page">
<h1>`)
	b.WriteString(htmlEsc(c.Meta.Title))
	b.WriteString(`</h1>
<div class="sub">`)
	b.WriteString(htmlEsc(c.Meta.Subtitle))
	if c.Meta.Subtitle != "" && c.Meta.Updated != "" { b.WriteString("  ·  ") }
	if c.Meta.Updated != "" { b.WriteString("updated " + htmlEsc(c.Meta.Updated)) }
	b.WriteString("</div>\n")

	for _, s := range c.Sections {
		b.WriteString(fmt.Sprintf("<section id=\"%s\"><h2>%s</h2>\n", htmlEsc(s.ID), htmlEsc(s.Name)))
		if s.Note != "" {
			b.WriteString("<p class=\"note\">" + htmlEsc(s.Note) + "</p>\n")
		}
		b.WriteString("<div class=\"tw\"><table><thead><tr><th>Date</th><th>Action</th><th>Detail</th></tr></thead><tbody>\n")
		for _, r := range s.Rows {
			cls := ""
			switch r.Marker() {
			case MarkerMilestone: cls = "milestone"
			case MarkerExposure:  cls = "exposure"
			case MarkerDeferred:  cls = "deferred"
			}
			b.WriteString(fmt.Sprintf("<tr class=\"%s\"><td class=\"date\">%s</td><td class=\"action\"><span class=\"marker\"></span>%s</td><td>%s</td></tr>\n",
				cls, htmlEsc(r.Date), htmlEsc(r.Action), htmlEsc(r.Detail)))
		}
		b.WriteString("</tbody></table></div></section>\n")
	}
	b.WriteString("</div></body></html>\n")
	return b.String()
}
