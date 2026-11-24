import React from 'react';
import { MessageSquareQuote, AlertTriangle, Sparkles, Clock, Smile } from 'lucide-react';

interface SpeechBubbleProps {
  phrase: string;
  priority?: number;
  emotion?: 'friendly' | 'excited' | 'alert' | 'happy' | string;
}

export const SpeechBubble: React.FC<SpeechBubbleProps> = ({
  phrase,
  priority = 4,
  emotion = 'friendly',
}) => {
  // Determine styling based on priority
  const isUrgent = priority === 1;
  const isProgress = priority === 2;
  const isTime = priority === 3;

  const getBorderColor = () => {
    if (isUrgent) return 'border-rose-500/60 bg-rose-950/80 text-rose-100 shadow-rose-950/50';
    if (isProgress) return 'border-amber-500/60 bg-amber-950/80 text-amber-100 shadow-amber-950/50';
    if (isTime) return 'border-sky-500/60 bg-sky-950/80 text-sky-100 shadow-sky-950/50';
    return 'border-indigo-500/40 bg-slate-900/90 text-slate-100 shadow-indigo-950/30';
  };

  const getIcon = () => {
    if (isUrgent) return <AlertTriangle className="w-3.5 h-3.5 text-rose-400 shrink-0" />;
    if (isProgress) return <Sparkles className="w-3.5 h-3.5 text-amber-400 shrink-0" />;
    if (isTime) return <Clock className="w-3.5 h-3.5 text-sky-400 shrink-0" />;
    return <Smile className="w-3.5 h-3.5 text-indigo-400 shrink-0" />;
  };

  return (
    <div className="relative mb-2 max-w-[280px] sm:max-w-xs mx-auto animate-in fade-in zoom-in-95 duration-200">
      <div
        className={`relative px-3.5 py-2.5 rounded-2xl border shadow-lg backdrop-blur-md text-center transition-all ${getBorderColor()}`}
      >
        <div className="flex items-center justify-center gap-1.5 mb-1 text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
          {getIcon()}
          <span>Маскот MAX</span>
        </div>
        <p className="text-xs sm:text-[13px] font-medium leading-snug break-words">
          "{phrase}"
        </p>

        {/* Speech Bubble pointer arrow pointing down to mascot */}
        <div className="absolute -bottom-2 left-1/2 -translate-x-1/2 w-0 h-0 border-x-8 border-x-transparent border-t-8 border-t-slate-800" />
      </div>
    </div>
  );
};
