import React from 'react';
import { parseMarkdown, InlineNode, BlockNode } from '../../utils/markdown';

interface FormattedMessageProps {
  content: string;
  isUser?: boolean;
  className?: string;
}

export const FormattedMessage: React.FC<FormattedMessageProps> = ({
  content,
  isUser = false,
  className = '',
}) => {
  const blocks = parseMarkdown(content);

  const renderInlines = (inlines?: InlineNode[]) => {
    if (!inlines || inlines.length === 0) return null;

    return inlines.map((node, idx) => {
      switch (node.type) {
        case 'bold':
          return (
            <strong key={idx} className="font-bold">
              {node.value}
            </strong>
          );
        case 'italic':
          return <em key={idx}>{node.value}</em>;
        case 'code':
          return (
            <code
              key={idx}
              className={`px-1 py-0.5 rounded text-[11px] font-mono ${
                isUser
                  ? 'bg-teal-700/80 text-white'
                  : 'bg-slate-200/90 text-teal-800 font-semibold'
              }`}
            >
              {node.value}
            </code>
          );
        case 'link':
          return (
            <a
              key={idx}
              href={node.url}
              target="_blank"
              rel="noreferrer"
              className={`underline transition-colors ${
                isUser ? 'text-teal-100 hover:text-white' : 'text-teal-600 hover:text-teal-800 font-medium'
              }`}
            >
              {node.text || node.url}
            </a>
          );
        case 'break':
          return <br key={idx} />;
        case 'text':
        default:
          return <React.Fragment key={idx}>{node.value}</React.Fragment>;
      }
    });
  };

  return (
    <div className={`space-y-2 leading-relaxed break-words ${className}`}>
      {blocks.map((block, bIdx) => {
        switch (block.type) {
          case 'heading': {
            if (block.level === 1) {
              return (
                <h2 key={bIdx} className="font-bold text-sm sm:text-base mt-2 mb-1">
                  {renderInlines(block.inlines)}
                </h2>
              );
            }
            if (block.level === 2) {
              return (
                <h3 key={bIdx} className="font-bold text-xs sm:text-sm mt-1.5 mb-1">
                  {renderInlines(block.inlines)}
                </h3>
              );
            }
            return (
              <h4 key={bIdx} className="font-bold text-xs mt-1 mb-0.5">
                {renderInlines(block.inlines)}
              </h4>
            );
          }

          case 'list': {
            if (block.ordered) {
              return (
                <ol key={bIdx} className="list-decimal pl-4 space-y-1 my-1">
                  {block.items?.map((item, iIdx) => (
                    <li key={iIdx}>{renderInlines(item)}</li>
                  ))}
                </ol>
              );
            }
            return (
              <ul key={bIdx} className="list-disc pl-4 space-y-1 my-1">
                {block.items?.map((item, iIdx) => (
                  <li key={iIdx}>{renderInlines(item)}</li>
                ))}
              </ul>
            );
          }

          case 'quote':
            return (
              <blockquote
                key={bIdx}
                className={`border-l-2 pl-2.5 my-1.5 italic ${
                  isUser ? 'border-teal-300 text-teal-100' : 'border-teal-500 text-slate-600'
                }`}
              >
                {renderInlines(block.inlines)}
              </blockquote>
            );

          case 'codeblock':
            return (
              <pre
                key={bIdx}
                className="my-1.5 p-2.5 rounded-xl bg-slate-900 text-slate-100 text-[11px] font-mono overflow-x-auto shadow-inner"
              >
                <code>{block.code}</code>
              </pre>
            );

          case 'paragraph':
          default:
            return <p key={bIdx}>{renderInlines(block.inlines)}</p>;
        }
      })}
    </div>
  );
};
