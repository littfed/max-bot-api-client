import test from 'node:test';
import assert from 'node:assert/strict';
import { parseMarkdown, parseInlines } from './markdown.ts';

test('parseInlines converts bold markdown to bold nodes', () => {
  const input = '**Риски? Какие риски, если ничего не делаешь?**';
  const nodes = parseInlines(input);
  assert.equal(nodes.length, 1);
  assert.equal(nodes[0].type, 'bold');
  assert.equal(nodes[0].value, 'Риски? Какие риски, если ничего не делаешь?');
});

test('parseInlines handles mixed text with inline code and bold', () => {
  const input = 'Обратите внимание на `config.json` и **важное примечание**.';
  const nodes = parseInlines(input);
  assert.equal(nodes[0].type, 'text');
  assert.equal(nodes[0].value, 'Обратите внимание на ');
  assert.equal(nodes[1].type, 'code');
  assert.equal(nodes[1].value, 'config.json');
  assert.equal(nodes[2].type, 'text');
  assert.equal(nodes[2].value, ' и ');
  assert.equal(nodes[3].type, 'bold');
  assert.equal(nodes[3].value, 'важное примечание');
});

test('parseMarkdown parses headings, bullet lists, and paragraphs with linebreaks', () => {
  const input = `### Главный совет

**Риски? Какие риски, если ничего не делаешь?**

Вот основные пункты:
• **Пункт 1**: запуск бота
• **Пункт 2**: подключение оплат

Удачи в запуске!`;

  const blocks = parseMarkdown(input);

  assert.equal(blocks.length, 5);

  // 1. Heading
  assert.equal(blocks[0].type, 'heading');
  assert.equal(blocks[0].level, 3);
  assert.equal(blocks[0].inlines[0].value, 'Главный совет');

  // 2. Bold Paragraph
  assert.equal(blocks[1].type, 'paragraph');
  assert.equal(blocks[1].inlines[0].type, 'bold');
  assert.equal(blocks[1].inlines[0].value, 'Риски? Какие риски, если ничего не делаешь?');

  // 3. Intro Paragraph
  assert.equal(blocks[2].type, 'paragraph');
  assert.equal(blocks[2].inlines[0].value, 'Вот основные пункты:');

  // 4. Bullet List
  assert.equal(blocks[3].type, 'list');
  assert.equal(blocks[3].ordered, false);
  assert.equal(blocks[3].items.length, 2);
  assert.equal(blocks[3].items[0][0].type, 'bold');
  assert.equal(blocks[3].items[0][0].value, 'Пункт 1');

  // 5. Outro Paragraph
  assert.equal(blocks[4].type, 'paragraph');
  assert.equal(blocks[4].inlines[0].value, 'Удачи в запуске!');
});
