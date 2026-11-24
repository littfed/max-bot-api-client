import React from 'react';

interface SpeechBubbleProps {
  phrase: string;
  priority?: number;
  emotion?: 'friendly' | 'excited' | 'alert' | 'happy' | string;
}

export const SpeechBubble: React.FC<SpeechBubbleProps> = ({
  phrase,
}) => {
  return (
    <div className="relative mb-2.5 max-w-[270px] sm:max-w-xs mx-auto animate-in fade-in duration-200">
      <div className="relative px-3.5 py-2.5 rounded-2xl bg-[#e0f2fe] border border-[#bae6fd] text-[#0369a1] shadow-sm text-center">
        <p className="text-xs sm:text-[13px] font-medium leading-snug">
          {phrase}
        </p>

        {/* Triangle Tail pointing down to robot mascot */}
        <div className="absolute -bottom-2 left-1/2 -translate-x-1/2 w-0 h-0 border-x-8 border-x-transparent border-t-8 border-t-[#e0f2fe]" />
      </div>
    </div>
  );
};
