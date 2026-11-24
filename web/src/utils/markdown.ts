export interface InlineNode {
  type: 'text' | 'bold' | 'italic' | 'code' | 'link' | 'break';
  value?: string;
  text?: string;
  url?: string;
}

export interface BlockNode {
  type: 'heading' | 'list' | 'codeblock' | 'quote' | 'paragraph';
  level?: number;
  ordered?: boolean;
  language?: string;
  code?: string;
  items?: InlineNode[][];
  inlines?: InlineNode[];
}

/**
 * Parses inline formatting: **bold**, *italic*, `code`, [link](url), \n breaks.
 */
export function parseInlines(text: string): InlineNode[] {
  const nodes: InlineNode[] = [];
  if (!text) return nodes;

  // Split lines to preserve intentional single line breaks
  const lines = text.split('\n');

  for (let lIdx = 0; lIdx < lines.length; lIdx++) {
    if (lIdx > 0) {
      nodes.push({ type: 'break' });
    }

    const line = lines[lIdx];
    let pos = 0;

    while (pos < line.length) {
      // Inline Code: `code`
      if (line[pos] === '`') {
        const end = line.indexOf('`', pos + 1);
        if (end !== -1) {
          nodes.push({
            type: 'code',
            value: line.substring(pos + 1, end),
          });
          pos = end + 1;
          continue;
        }
      }

      // Bold: **text** or __text__
      if (
        (line.startsWith('**', pos) || line.startsWith('__', pos)) &&
        pos + 2 < line.length
      ) {
        const marker = line.substring(pos, pos + 2);
        const end = line.indexOf(marker, pos + 2);
        if (end !== -1) {
          nodes.push({
            type: 'bold',
            value: line.substring(pos + 2, end),
          });
          pos = end + 2;
          continue;
        }
      }

      // Italic: *text* or _text_ (when not followed/preceded by another *)
      if (
        (line[pos] === '*' || line[pos] === '_') &&
        pos + 1 < line.length &&
        line[pos + 1] !== line[pos]
      ) {
        const marker = line[pos];
        const end = line.indexOf(marker, pos + 1);
        if (end !== -1 && end > pos + 1 && line[end + 1] !== marker) {
          nodes.push({
            type: 'italic',
            value: line.substring(pos + 1, end),
          });
          pos = end + 1;
          continue;
        }
      }

      // Markdown Link: [text](url)
      if (line[pos] === '[') {
        const closeBracket = line.indexOf(']', pos + 1);
        if (
          closeBracket !== -1 &&
          closeBracket + 1 < line.length &&
          line[closeBracket + 1] === '('
        ) {
          const closeParen = line.indexOf(')', closeBracket + 2);
          if (closeParen !== -1) {
            const linkText = line.substring(pos + 1, closeBracket);
            const linkUrl = line.substring(closeBracket + 2, closeParen);
            nodes.push({
              type: 'link',
              text: linkText,
              url: linkUrl,
            });
            pos = closeParen + 1;
            continue;
          }
        }
      }

      // Regular text up to next special char
      let nextSpecial = pos + 1;
      while (
        nextSpecial < line.length &&
        line[nextSpecial] !== '`' &&
        line[nextSpecial] !== '*' &&
        line[nextSpecial] !== '_' &&
        line[nextSpecial] !== '['
      ) {
        nextSpecial++;
      }

      nodes.push({
        type: 'text',
        value: line.substring(pos, nextSpecial),
      });
      pos = nextSpecial;
    }
  }

  return nodes;
}

/**
 * Parses raw text into structured blocks (code blocks, headings, lists, quotes, paragraphs).
 */
export function parseMarkdown(raw: string): BlockNode[] {
  if (!raw || !raw.trim()) return [];

  // Normalize Windows CRLF to LF
  const text = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n');

  const blocks: BlockNode[] = [];
  const lines = text.split('\n');
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // 1. Code Block: ```lang
    if (line.trim().startsWith('```')) {
      const lang = line.trim().replace(/^```/, '').trim();
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith('```')) {
        codeLines.push(lines[i]);
        i++;
      }
      if (i < lines.length && lines[i].trim().startsWith('```')) {
        i++; // skip closing ```
      }
      blocks.push({
        type: 'codeblock',
        language: lang,
        code: codeLines.join('\n'),
      });
      continue;
    }

    // 2. Empty Line -> Skip
    if (!line.trim()) {
      i++;
      continue;
    }

    // 3. Headings: #, ##, ###
    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      blocks.push({
        type: 'heading',
        level: Math.min(level, 3),
        inlines: parseInlines(headingMatch[2].trim()),
      });
      i++;
      continue;
    }

    // 4. Blockquote: > text
    if (line.startsWith('>')) {
      const quoteLines: string[] = [];
      while (i < lines.length && lines[i].startsWith('>')) {
        quoteLines.push(lines[i].replace(/^>\s?/, ''));
        i++;
      }
      blocks.push({
        type: 'quote',
        inlines: parseInlines(quoteLines.join('\n')),
      });
      continue;
    }

    // 5. Unordered List: -, *, •
    const bulletMatch = line.match(/^(\s*[-*•]\s+)(.+)$/);
    if (bulletMatch) {
      const items: InlineNode[][] = [];
      while (i < lines.length) {
        const itemMatch = lines[i].match(/^(\s*[-*•]\s+)(.+)$/);
        if (itemMatch) {
          items.push(parseInlines(itemMatch[2].trim()));
          i++;
        } else if (lines[i].trim() === '') {
          // Check if next line continues the list
          if (i + 1 < lines.length && lines[i + 1].match(/^(\s*[-*•]\s+)(.+)$/)) {
            i++;
          } else {
            break;
          }
        } else {
          break;
        }
      }
      blocks.push({
        type: 'list',
        ordered: false,
        items,
      });
      continue;
    }

    // 6. Ordered List: 1. Item
    const numMatch = line.match(/^(\s*\d+[.)]\s+)(.+)$/);
    if (numMatch) {
      const items: InlineNode[][] = [];
      while (i < lines.length) {
        const itemMatch = lines[i].match(/^(\s*\d+[.)]\s+)(.+)$/);
        if (itemMatch) {
          items.push(parseInlines(itemMatch[2].trim()));
          i++;
        } else if (lines[i].trim() === '') {
          if (i + 1 < lines.length && lines[i + 1].match(/^(\s*\d+[.)]\s+)(.+)$/)) {
            i++;
          } else {
            break;
          }
        } else {
          break;
        }
      }
      blocks.push({
        type: 'list',
        ordered: true,
        items,
      });
      continue;
    }

    // 7. Regular Paragraph (accumulate lines until blank line or special block)
    const paraLines: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() &&
      !lines[i].trim().startsWith('```') &&
      !lines[i].match(/^(#{1,6})\s+(.+)$/) &&
      !lines[i].startsWith('>') &&
      !lines[i].match(/^(\s*[-*•]\s+)(.+)$/) &&
      !lines[i].match(/^(\s*\d+[.)]\s+)(.+)$/)
    ) {
      paraLines.push(lines[i]);
      i++;
    }

    blocks.push({
      type: 'paragraph',
      inlines: parseInlines(paraLines.join('\n')),
    });
  }

  return blocks;
}
