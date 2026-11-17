#!/usr/bin/env bun
// cdf — ClaudeDocFormat reader/editor
//
// Usage:
//   cdf file.cdf              open in TUI viewer/editor (default = view)
//   cdf view file.cdf         open in TUI viewer/editor
//   cdf new "Title"           print an empty CDF skeleton to stdout
//   cdf validate file.cdf     check schema, exit 0 if valid, 1 if not
//   cdf export file.cdf       print an HTML export to stdout
//   cdf --version             print version
//   cdf --help                show help
//
// Compile to standalone binary:
//   bun build --compile ~/.claude/skills/CDF/Tools/cdf.ts --outfile ~/bin/cdf
//
// No runtime dependencies beyond Bun. TUI uses ANSI escape codes and
// box-drawing characters; the same rendering on every terminal.

import { readFileSync, writeFileSync, existsSync, mkdtempSync, rmSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const CDF_VERSION = 1;
const CDF_TOOL_VERSION = '1.0.1';

// ─────────────────────────── types ───────────────────────────

interface Row { date: string; action: string; detail?: string; [k: string]: any; }
interface Section { id: string; name: string; note?: string; rows: Row[]; [k: string]: any; }
interface CDF {
  meta: { cdf_version: number; title: string; subtitle?: string; updated: string; [k: string]: any; };
  sections: Section[];
  [k: string]: any;
}

// ─────────────────────────── ansi ───────────────────────────

const ESC = '\x1b[';
const ansi = {
  reset:     ESC + '0m',
  bold:      ESC + '1m',
  dim:       ESC + '2m',
  underline: ESC + '4m',
  reverse:   ESC + '7m',
  fg:  (r: number, g: number, b: number) => `${ESC}38;2;${r};${g};${b}m`,
  bg:  (r: number, g: number, b: number) => `${ESC}48;2;${r};${g};${b}m`,
  clr:      ESC + '2J' + ESC + 'H',           // clear screen + home
  cur:  (r: number, c: number) => `${ESC}${r};${c}H`,
  hide:     ESC + '?25l',
  show:     ESC + '?25h',
  altOn:    ESC + '?1049h',                    // alternate screen buffer
  altOff:   ESC + '?1049l',
  clrLine:  ESC + '2K',
  clrDown:  ESC + '0J',
};
const C = {
  ink:    ansi.reset,
  muted:  ansi.fg(122, 128, 144),
  accent: ansi.fg(79, 209, 197),
  warn:   ansi.fg(212, 169, 74),
  danger: ansi.fg(224, 112, 112),
};

// ─────────────────────────── io helpers ───────────────────────────

function readCdf(path: string): CDF {
  const text = readFileSync(path, 'utf8');
  const parsed = JSON.parse(text);
  validateCdf(parsed);   // throws on schema failure
  return parsed;
}
function writeCdf(path: string, cdf: CDF): void {
  cdf.meta.updated = new Date().toISOString().slice(0, 10);
  writeFileSync(path, JSON.stringify(cdf, null, 2) + '\n');
}
function validateCdf(c: any): void {
  if (!c || typeof c !== 'object') throw new Error('not a JSON object');
  if (!c.meta || typeof c.meta !== 'object') throw new Error('missing meta');
  if (c.meta.cdf_version !== 1) throw new Error(`unsupported cdf_version: ${c.meta.cdf_version}`);
  if (!c.meta.title) throw new Error('missing meta.title');
  if (!c.meta.updated) throw new Error('missing meta.updated');
  if (!Array.isArray(c.sections)) throw new Error('missing sections array');
  const ids = new Set<string>();
  for (const s of c.sections) {
    if (!s.id || !s.name) throw new Error(`section missing id or name: ${JSON.stringify(s)}`);
    if (ids.has(s.id)) throw new Error(`duplicate section id: ${s.id}`);
    ids.add(s.id);
    if (!Array.isArray(s.rows)) throw new Error(`section ${s.id} missing rows array`);
  }
}
function skeleton(title: string): CDF {
  return {
    meta: { cdf_version: 1, title, subtitle: '', updated: new Date().toISOString().slice(0, 10) },
    sections: [{ id: 'section-1', name: 'New section', note: '', rows: [
      { date: new Date().toISOString().slice(0, 10), action: 'First action', detail: '' }
    ]}]
  };
}

// ─────────────────────────── row classification ───────────────────────────

function markerColor(row: Row): string {
  const a = (row.action || '').trim();
  if (a.startsWith('MILESTONE:')) return C.accent;
  if (a.startsWith('EXPOSURE:')) return C.danger;
  const d = (row.date || '').trim();
  if (d === '—' || d === '-') return C.warn;
  return C.muted;
}

// ─────────────────────────── terminal ───────────────────────────

function termSize(): { w: number; h: number } {
  return { w: process.stdout.columns || 80, h: process.stdout.rows || 24 };
}
function out(s: string): void { process.stdout.write(s); }

// wcwidth-ish: treat everything as width 1 except CJK-ish ranges (width 2) and control (width 0).
// Good enough for terminal ops; the tool renders ASCII/English content dominantly.
function cellWidth(str: string): number {
  let w = 0;
  for (const ch of str) {
    const cp = ch.codePointAt(0)!;
    if (cp < 0x20) continue;
    if (cp >= 0x1100 && (
      cp <= 0x115f ||
      (cp >= 0x2e80 && cp <= 0x303e) ||
      (cp >= 0x3041 && cp <= 0x33ff) ||
      (cp >= 0x3400 && cp <= 0x4dbf) ||
      (cp >= 0x4e00 && cp <= 0x9fff) ||
      (cp >= 0xa000 && cp <= 0xa4cf) ||
      (cp >= 0xac00 && cp <= 0xd7a3) ||
      (cp >= 0xf900 && cp <= 0xfaff) ||
      (cp >= 0xfe30 && cp <= 0xfe4f) ||
      (cp >= 0xff00 && cp <= 0xff60) ||
      (cp >= 0xffe0 && cp <= 0xffe6)
    )) w += 2;
    else w += 1;
  }
  return w;
}

// Slice a string to exactly `width` display cells (no ANSI awareness needed — content strings are ANSI-free).
function slice(str: string, width: number): string {
  if (cellWidth(str) <= width) return str + ' '.repeat(width - cellWidth(str));
  let out = '', w = 0;
  for (const ch of str) {
    const cw = cellWidth(ch);
    if (w + cw > width - 1) break;
    out += ch; w += cw;
  }
  return out + '…' + ' '.repeat(Math.max(0, width - w - 1));
}

// Wrap a string into lines of exactly `width` display cells, at word boundaries where possible.
function wrap(str: string, width: number): string[] {
  if (!str) return [' '.repeat(width)];
  const lines: string[] = [];
  const paragraphs = str.split(/\r?\n/);
  for (const para of paragraphs) {
    if (para === '') { lines.push(' '.repeat(width)); continue; }
    const words = para.split(/(\s+)/);
    let cur = '';
    for (const w of words) {
      if (cellWidth(cur) + cellWidth(w) <= width) { cur += w; continue; }
      // word doesn't fit; flush cur, start new
      if (cur.trim()) lines.push(padRight(cur, width));
      cur = w.trimStart();
      // if the word alone exceeds width, hard-break it
      while (cellWidth(cur) > width) {
        let take = '', tw = 0;
        for (const ch of cur) {
          const cw = cellWidth(ch);
          if (tw + cw > width) break;
          take += ch; tw += cw;
        }
        lines.push(padRight(take, width));
        cur = cur.slice(take.length);
      }
    }
    lines.push(padRight(cur, width));
  }
  return lines;
}
function padRight(s: string, width: number): string {
  const w = cellWidth(s);
  return w >= width ? s : s + ' '.repeat(width - w);
}

// ─────────────────────────── layout ───────────────────────────

function computeColWidths(rows: Row[], available: number): [number, number, number] {
  // Row template: "  │ D │ A │ De │" — total non-content chars = 12 (2 margin + 4 verticals + 6 inner spaces).
  // So dW + aW + deW must fit in (available - 12).
  const dateW = 11;
  const contentBudget = Math.max(available - 12, 30);
  const remaining = Math.max(contentBudget - dateW, 20);
  const actionW = Math.max(20, Math.floor(remaining * 0.32));
  const detailW = Math.max(20, remaining - actionW);
  return [dateW, actionW, detailW];
}

// ─────────────────────────── render ───────────────────────────

interface Cursor { section: number; row: number; cell: 'date' | 'action' | 'detail'; }

function renderScreen(cdf: CDF, cursor: Cursor, dirty: boolean, filePath: string, mode: 'view' | 'edit', message: string): void {
  const { w, h } = termSize();
  out(ansi.clr);

  // ── header ──────────────
  const title = cdf.meta.title || 'Untitled';
  const dirtyMark = dirty ? ` ${C.warn}●${C.ink}` : '';
  const rightMeta = `${C.muted}${cdf.meta.updated || ''}${C.ink}`;
  const leftHeader = `${ansi.bold}${title}${ansi.reset}${dirtyMark}`;
  const leftW = cellWidth(title) + (dirty ? 2 : 0);
  const rightW = cellWidth(cdf.meta.updated || '');
  const pad = Math.max(1, w - leftW - rightW);
  out(leftHeader + ' '.repeat(pad) + rightMeta + '\n');

  if (cdf.meta.subtitle) {
    out(C.muted + slice(cdf.meta.subtitle, w) + C.ink + '\n');
  }

  // rule
  out(C.muted + '─'.repeat(w) + C.ink + '\n');

  // ── section navigator ──────────────
  const navParts: string[] = [];
  cdf.sections.forEach((s, i) => {
    const label = `${i + 1} ${s.name}`;
    if (i === cursor.section) navParts.push(`${C.accent}${ansi.bold}[${label}]${ansi.reset}`);
    else navParts.push(`${C.muted}${label}${C.ink}`);
  });
  const navLine = navParts.join(`  ${C.muted}·${C.ink}  `);
  // print truncated if it doesn't fit
  out(navLine + '\n');
  out(C.muted + '─'.repeat(w) + C.ink + '\n');

  // ── current section content ──────────────
  const s = cdf.sections[cursor.section];
  if (!s) {
    out('\n  ' + C.muted + '(no sections — press A to add one)' + C.ink + '\n');
  } else {
    out(`\n  ${C.accent}${ansi.bold}${s.name}${ansi.reset}\n`);
    if (s.note) {
      const noteWidth = Math.min(w - 4, 90);
      for (const line of wrap(s.note, noteWidth)) {
        out(`  ${C.muted}${line}${C.ink}\n`);
      }
    }
    out('\n');

    // ── table ──────────────
    const tableW = w - 2;  // 1ch left/right margin
    const [dW, aW, deW] = computeColWidths(s.rows, tableW);
    const totalW = dW + aW + deW + 4;   // 4 = 3 inner borders + 1 (the +2 corners are inside)

    // top border
    out(`  ${C.muted}┌${'─'.repeat(dW + 2)}┬${'─'.repeat(aW + 2)}┬${'─'.repeat(deW + 2)}┐${C.ink}\n`);
    // header row
    out(`  ${C.muted}│ ${ansi.bold}${slice('DATE', dW)}${ansi.reset}${C.muted} │ ${ansi.bold}${slice('ACTION', aW)}${ansi.reset}${C.muted} │ ${ansi.bold}${slice('DETAIL', deW)}${ansi.reset}${C.muted} │${C.ink}\n`);
    out(`  ${C.muted}├${'─'.repeat(dW + 2)}┼${'─'.repeat(aW + 2)}┼${'─'.repeat(deW + 2)}┤${C.ink}\n`);

    // rows
    s.rows.forEach((r, ri) => {
      const mark = markerColor(r);
      const isCurrentRow = ri === cursor.row;
      const dateLines = wrap(r.date || '', dW);
      const actionText = r.action || '';
      const actionLines = wrap(actionText, aW - 2);   // -2 for "● " marker prefix
      const detailLines = wrap(r.detail || '', deW);
      const rowHeight = Math.max(dateLines.length, actionLines.length, detailLines.length);

      for (let li = 0; li < rowHeight; li++) {
        const dl = dateLines[li] ?? ' '.repeat(dW);
        const al = actionLines[li] ?? ' '.repeat(aW - 2);
        const de = detailLines[li] ?? ' '.repeat(deW);

        const cellHi = (which: 'date' | 'action' | 'detail') =>
          isCurrentRow && cursor.cell === which ? ansi.reverse : '';
        const cellHiOff = ansi.reset + (isCurrentRow ? '' : ''); // reset

        // date cell
        const dateHi = cellHi('date');
        // action cell — marker on first line only
        const marker = li === 0 ? `${mark}●${C.ink} ` : '  ';
        const actionHi = cellHi('action');
        // detail cell
        const detailHi = cellHi('detail');

        out(`  ${C.muted}│ ${dateHi}${C.muted}${dl}${ansi.reset}${C.muted} │ ${marker}${actionHi}${al}${ansi.reset}${C.muted} │ ${detailHi}${C.ink}${de}${ansi.reset}${C.muted} │${C.ink}\n`);
      }
      // divider between rows (except last)
      if (ri < s.rows.length - 1) {
        out(`  ${C.muted}├${'─'.repeat(dW + 2)}┼${'─'.repeat(aW + 2)}┼${'─'.repeat(deW + 2)}┤${C.ink}\n`);
      }
    });

    // bottom border
    if (s.rows.length === 0) {
      out(`  ${C.muted}│ ${slice('(empty — press a to add a row)', dW + aW + deW + 6)} │${C.ink}\n`);
    }
    out(`  ${C.muted}└${'─'.repeat(dW + 2)}┴${'─'.repeat(aW + 2)}┴${'─'.repeat(deW + 2)}┘${C.ink}\n`);
  }

  // ── status bar ──────────────
  // move to bottom row
  const { h: h2 } = termSize();
  out(ansi.cur(h2, 1) + ansi.clrLine);
  const help = `${C.muted}↑↓ row  ←→ cell  n/p section  ${C.accent}⏎${C.muted} edit  a add  d del  A new-sec  s save  ?${C.muted} help  q quit${C.ink}`;
  const status = message ? `${C.warn}${message}${C.ink}  ` : '';
  const shortPath = filePath.length > 40 ? '…' + filePath.slice(-38) : filePath;
  out(help);
  const rightStatus = `${status}${C.muted}${shortPath}${C.ink}`;
  const helpW = cellWidth('↑↓ row  ←→ cell  n/p section  ⏎ edit  a add  d del  A new-sec  s save  ? help  q quit');
  const rightPad = Math.max(1, w - helpW - cellWidth(shortPath) - cellWidth(message));
  out(' '.repeat(rightPad) + rightStatus);
}

// ─────────────────────────── input ───────────────────────────

async function readKey(): Promise<string> {
  return new Promise(resolve => {
    const onData = (buf: Buffer) => {
      process.stdin.off('data', onData);
      resolve(buf.toString('utf8'));
    };
    process.stdin.on('data', onData);
  });
}

// Prompt for a single-line answer at bottom of screen (blocking-style).
async function prompt(msg: string): Promise<string> {
  const { h, w } = termSize();
  out(ansi.cur(h, 1) + ansi.clrLine + C.accent + msg + C.ink + ' ');
  out(ansi.show);
  process.stdin.setRawMode(false);
  const line = await new Promise<string>(resolve => {
    let buf = '';
    const onData = (d: Buffer) => {
      const s = d.toString('utf8');
      if (s === '\n' || s === '\r') { process.stdin.off('data', onData); resolve(buf); }
      else if (s === '\x03' || s === '\x1b') { process.stdin.off('data', onData); resolve(''); }
      else if (s === '\x7f' || s === '\b') { buf = buf.slice(0, -1); out('\b \b'); }
      else { buf += s; out(s); }
    };
    process.stdin.on('data', onData);
  });
  process.stdin.setRawMode(true);
  out(ansi.hide);
  return line.trim();
}

async function confirm(msg: string): Promise<boolean> {
  const a = await prompt(msg + ' [y/N]:');
  return /^y(es)?$/i.test(a);
}

// ─────────────────────────── $EDITOR delegation ───────────────────────────

function editInEditor(initial: string, hint: string): string {
  const editor = process.env.EDITOR || process.env.VISUAL || 'vi';
  const dir = mkdtempSync(join(tmpdir(), 'cdf-'));
  const file = join(dir, `edit.${hint}.txt`);
  writeFileSync(file, initial);
  // Restore terminal to cooked mode + main screen while editor runs
  process.stdin.setRawMode(false);
  out(ansi.altOff + ansi.show);
  const res = spawnSync(editor, [file], { stdio: 'inherit' });
  out(ansi.altOn + ansi.hide);
  process.stdin.setRawMode(true);
  if (res.status !== 0) { rmSync(dir, { recursive: true, force: true }); return initial; }
  const newVal = readFileSync(file, 'utf8').replace(/\n$/, '');
  rmSync(dir, { recursive: true, force: true });
  return newVal;
}

// ─────────────────────────── main TUI loop ───────────────────────────

async function viewer(filePath: string): Promise<void> {
  let cdf = readCdf(filePath);
  let cursor: Cursor = { section: 0, row: 0, cell: 'action' };
  let dirty = false;
  let message = '';
  const mode: 'view' | 'edit' = 'view';

  // enter alt-screen, hide cursor, raw mode
  out(ansi.altOn + ansi.hide);
  process.stdin.setRawMode(true);
  process.stdin.resume();

  const cleanup = () => {
    process.stdin.setRawMode(false);
    out(ansi.show + ansi.altOff);
  };
  const bail = (code = 0) => { cleanup(); process.exit(code); };
  process.on('SIGINT', () => bail(130));
  process.on('SIGTERM', () => bail(143));
  process.on('uncaughtException', e => { cleanup(); console.error(e); process.exit(1); });

  // handle terminal resize
  process.on('SIGWINCH', () => { renderScreen(cdf, cursor, dirty, filePath, mode, message); });

  const currentSection = () => cdf.sections[cursor.section];
  const currentRow = () => currentSection()?.rows[cursor.row];
  const clampCursor = () => {
    if (cursor.section >= cdf.sections.length) cursor.section = Math.max(0, cdf.sections.length - 1);
    const s = currentSection();
    if (!s) return;
    if (cursor.row >= s.rows.length) cursor.row = Math.max(0, s.rows.length - 1);
    if (cursor.row < 0) cursor.row = 0;
  };

  const setMsg = (m: string) => { message = m; setTimeout(() => { if (message === m) { message = ''; renderScreen(cdf, cursor, dirty, filePath, mode, message); } }, 2500); };

  renderScreen(cdf, cursor, dirty, filePath, mode, message);

  while (true) {
    const key = await readKey();
    message = '';

    // arrow keys
    if (key === '\x1b[A') {          // up
      if (cursor.row > 0) cursor.row--;
    } else if (key === '\x1b[B') {   // down
      const s = currentSection();
      if (s && cursor.row < s.rows.length - 1) cursor.row++;
    } else if (key === '\x1b[C') {   // right
      cursor.cell = cursor.cell === 'date' ? 'action' : cursor.cell === 'action' ? 'detail' : 'detail';
    } else if (key === '\x1b[D') {   // left
      cursor.cell = cursor.cell === 'detail' ? 'action' : cursor.cell === 'action' ? 'date' : 'date';
    } else if (key === 'n' || key === '\x1b[6~') {   // next section (n or PgDn)
      if (cursor.section < cdf.sections.length - 1) { cursor.section++; cursor.row = 0; }
    } else if (key === 'p' || key === '\x1b[5~') {   // prev section
      if (cursor.section > 0) { cursor.section--; cursor.row = 0; }
    } else if (/^[1-9]$/.test(key)) {                // numeric section jump
      const idx = parseInt(key, 10) - 1;
      if (idx < cdf.sections.length) { cursor.section = idx; cursor.row = 0; }
    } else if (key === '\r' || key === '\n' || key === 'e') {   // edit cell in $EDITOR
      const row = currentRow(); const sec = currentSection();
      if (!sec) { setMsg('no section'); }
      else if (!row && cursor.cell !== 'date' && cursor.cell !== 'action' && cursor.cell !== 'detail') { setMsg('no row'); }
      else if (!row) { setMsg('no row — press a to add'); }
      else {
        const initial = row[cursor.cell] || '';
        const updated = editInEditor(initial, cursor.cell);
        if (updated !== initial) { row[cursor.cell] = updated; dirty = true; setMsg('edited'); }
      }
    } else if (key === 'a') {                        // add row
      const s = currentSection();
      if (s) {
        s.rows.splice(cursor.row + 1, 0, { date: new Date().toISOString().slice(0, 10), action: '', detail: '' });
        cursor.row = Math.min(cursor.row + 1, s.rows.length - 1);
        dirty = true; setMsg('row added');
      }
    } else if (key === 'd') {                        // delete row
      const s = currentSection();
      if (s && s.rows.length > 0) {
        if (await confirm('delete row?')) { s.rows.splice(cursor.row, 1); dirty = true; setMsg('row deleted'); clampCursor(); }
      }
    } else if (key === 'A') {                        // add section
      const name = await prompt('section name:');
      if (name) {
        const id = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'section-' + Date.now().toString(36);
        cdf.sections.push({ id, name, note: '', rows: [] });
        cursor.section = cdf.sections.length - 1; cursor.row = 0;
        dirty = true; setMsg('section added');
      }
    } else if (key === 'D') {                        // delete section
      if (cdf.sections.length > 0 && await confirm(`delete section "${cdf.sections[cursor.section].name}"?`)) {
        cdf.sections.splice(cursor.section, 1);
        cursor.section = Math.max(0, cursor.section - 1); cursor.row = 0;
        dirty = true; setMsg('section deleted');
      }
    } else if (key === 'N') {                        // edit section note
      const s = currentSection();
      if (s) {
        const updated = editInEditor(s.note || '', 'note');
        if (updated !== (s.note || '')) { s.note = updated; dirty = true; setMsg('note edited'); }
      }
    } else if (key === 'R') {                        // rename section
      const s = currentSection();
      if (s) {
        const nv = await prompt(`rename "${s.name}" to:`);
        if (nv) { s.name = nv; dirty = true; setMsg('renamed'); }
      }
    } else if (key === 'T') {                        // edit title
      const nv = await prompt(`title:`);
      if (nv) { cdf.meta.title = nv; dirty = true; setMsg('title updated'); }
    } else if (key === 's') {                        // save
      try { writeCdf(filePath, cdf); dirty = false; setMsg('saved'); }
      catch (e: any) { setMsg('save failed: ' + e.message); }
    } else if (key === 'q' || key === '\x03') {      // quit
      if (dirty) {
        if (await confirm('unsaved changes — quit anyway?')) bail(0);
        else { renderScreen(cdf, cursor, dirty, filePath, mode, message); continue; }
      }
      bail(0);
    } else if (key === '?' || key === 'h') {         // help
      showHelp();
      await readKey();
    }

    clampCursor();
    renderScreen(cdf, cursor, dirty, filePath, mode, message);
  }
}

function showHelp(): void {
  out(ansi.clr);
  out(`
  ${ansi.bold}${C.accent}CDF viewer/editor — keybindings${ansi.reset}

    ${C.muted}navigation${ansi.reset}
      ↑ ↓            move between rows
      ← →            move between cells (date / action / detail)
      n / PgDn       next section
      p / PgUp       prev section
      1 - 9          jump to section by number

    ${C.muted}editing${ansi.reset}
      Enter / e      edit current cell in $EDITOR (${process.env.EDITOR || 'vi'})
      a              add a row below current
      d              delete current row (confirm)
      A              add a new section (prompts for name)
      D              delete current section (confirm)
      N              edit current section's note in $EDITOR
      R              rename current section
      T              rename the document (edit meta.title)

    ${C.muted}file${ansi.reset}
      s              save to disk (JSON, pretty-printed)
      q              quit (prompts if unsaved)

    ${C.muted}other${ansi.reset}
      ?  or  h       this help
      Ctrl-C         quit (prompts if unsaved)

  ${C.muted}Row marker color is auto-classified:
    teal = MILESTONE:...   red = EXPOSURE:...   amber = date "—"${ansi.reset}

  ${C.muted}press any key to return${ansi.reset}
`);
}

// ─────────────────────────── html export ───────────────────────────

function toHtml(cdf: CDF): string {
  const esc = (s: string) => (s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  const sections = cdf.sections.map(s => {
    const rows = s.rows.map(r => {
      const a = (r.action || '').trim();
      const d = (r.date || '').trim();
      let cls = '';
      if (a.startsWith('MILESTONE:')) cls = 'milestone';
      else if (a.startsWith('EXPOSURE:')) cls = 'exposure';
      else if (d === '—' || d === '-') cls = 'deferred';
      return `      <tr class="${cls}"><td class="date">${esc(r.date)}</td><td class="action"><span class="marker"></span>${esc(r.action)}</td><td>${esc(r.detail || '')}</td></tr>`;
    }).join('\n');
    return `  <section id="${esc(s.id)}"><h2>${esc(s.name)}</h2>${s.note ? `<p class="note">${esc(s.note)}</p>` : ''}
    <div class="tw"><table><thead><tr><th>Date</th><th>Action</th><th>Detail</th></tr></thead><tbody>
${rows}
    </tbody></table></div></section>`;
  }).join('\n');

  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>${esc(cdf.meta.title)}</title>
<style>
:root{--bg:#0d0e11;--surface:#14161b;--border:#2a2e37;--ink:#e8ebef;--muted:#7a8090;--accent:#4fd1c5;--warn:#d4a94a;--danger:#e07070}
*{box-sizing:border-box}html,body{background:var(--bg)}
body{color:var(--ink);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Helvetica Neue",Arial,sans-serif;font-size:15px;line-height:1.55;margin:0;-webkit-font-smoothing:antialiased}
.page{max-width:960px;margin:0 auto;padding:3rem 1.5rem 6rem}
h1{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-size:1.55rem;font-weight:700;margin:0 0 .3rem}
.sub{color:var(--muted);font-size:.9rem;margin-bottom:1.25rem;padding-bottom:1.25rem;border-bottom:1px solid var(--border)}
section{margin-top:2.75rem}h2{font-family:ui-monospace,"SF Mono","Cascadia Code",Menlo,Consolas,monospace;font-size:1.1rem;color:var(--accent);margin:0 0 .4rem}
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
<h1>${esc(cdf.meta.title)}</h1>
<div class="sub">${esc(cdf.meta.subtitle || '')}${cdf.meta.subtitle && cdf.meta.updated ? '  ·  ' : ''}${cdf.meta.updated ? 'updated ' + esc(cdf.meta.updated) : ''}</div>
${sections}
</div></body></html>
`;
}

// ─────────────────────────── entry ───────────────────────────

function usage(): void {
  process.stderr.write(`cdf ${CDF_TOOL_VERSION} — ClaudeDocFormat reader/editor

Usage:
  cdf file.cdf              open in TUI viewer/editor
  cdf view file.cdf         same
  cdf new "Title"           print a skeleton CDF to stdout
  cdf validate file.cdf     check schema (exit 0 ok, 1 fail)
  cdf export file.cdf       print HTML export to stdout
  cdf --version | --help

Keybindings (in the viewer): ? or h
`);
}

async function main(): Promise<void> {
  const argv = process.argv.slice(2);
  if (argv.length === 0 || argv[0] === '--help' || argv[0] === '-h') { usage(); process.exit(0); }
  if (argv[0] === '--version') { process.stdout.write('cdf ' + CDF_TOOL_VERSION + ' (cdf_version ' + CDF_VERSION + ')\n'); process.exit(0); }

  if (argv[0] === 'new') {
    const title = argv[1] || 'Untitled';
    process.stdout.write(JSON.stringify(skeleton(title), null, 2) + '\n');
    return;
  }
  if (argv[0] === 'validate') {
    const path = argv[1]; if (!path) { usage(); process.exit(2); }
    try { readCdf(path); console.log('ok: ' + path); process.exit(0); }
    catch (e: any) { console.error('invalid: ' + e.message); process.exit(1); }
  }
  if (argv[0] === 'export') {
    const path = argv[1]; if (!path) { usage(); process.exit(2); }
    const cdf = readCdf(path);
    process.stdout.write(toHtml(cdf));
    return;
  }
  if (argv[0] === 'view' || argv[0].endsWith('.cdf') || existsSync(argv[0])) {
    const path = argv[0] === 'view' ? argv[1] : argv[0];
    if (!path || !existsSync(path)) { console.error('file not found: ' + path); process.exit(2); }
    if (!process.stdout.isTTY || !process.stdin.isTTY) {
      // Not attached to a terminal — dump a readable summary instead.
      const cdf = readCdf(path);
      console.error(`(no tty — summary of ${path})`);
      for (const s of cdf.sections) {
        console.log(`\n### ${s.name}`);
        if (s.note) console.log(s.note);
        for (const r of s.rows) console.log(`  ${r.date}  ${r.action}${r.detail ? ' — ' + r.detail : ''}`);
      }
      return;
    }
    await viewer(path);
    return;
  }

  usage();
  process.exit(2);
}

main().catch(e => { console.error(e); process.exit(1); });
