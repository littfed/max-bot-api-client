import React, { useState } from 'react';
import { SpeechBubble } from './SpeechBubble';
import { MascotState } from '../../types';

interface MascotRendererProps {
  mascot: MascotState;
  onTap: () => void;
}

export const MascotRenderer: React.FC<MascotRendererProps> = ({ mascot, onTap }) => {
  const [isBouncing, setIsBouncing] = useState(false);

  const handleTap = () => {
    // Micro-bounce reaction
    setIsBouncing(true);
    setTimeout(() => setIsBouncing(false), 300);

    // Haptic feedback if in Telegram Mini App
    if (window.Telegram?.WebApp?.initData) {
      try {
        (window.Telegram.WebApp as any)?.HapticFeedback?.impactOccurred?.('light');
      } catch (_) {}
    }

    onTap();
  };

  return (
    <div className="flex flex-col items-center justify-center select-none py-1">
      {/* Speech Cloud above mascot */}
      <SpeechBubble
        phrase={mascot.phrase}
        priority={mascot.priority}
        emotion={mascot.emotion}
      />

      {/* Mascot character with floating animation and tap bounce */}
      <div
        onClick={handleTap}
        className={`relative cursor-pointer group transition-transform duration-150 ${
          isBouncing ? 'scale-90' : 'hover:scale-105 active:scale-95'
        }`}
        title="Нажмите на маскота, чтобы пообщаться!"
      >
        {/* Ambient glow behind mascot */}
        <div className="absolute inset-0 bg-indigo-500/20 rounded-full blur-xl group-hover:bg-indigo-500/30 transition-all pointer-events-none" />

        <div className="relative animate-float-slow">
          <svg
            className="w-24 h-24 sm:w-28 sm:h-28 drop-shadow-2xl"
            viewBox="0 0 120 120"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            {/* Robot Head Outer */}
            <rect
              x="20"
              y="28"
              width="80"
              height="72"
              rx="24"
              fill="url(#mascotGrad)"
              stroke="#6366f1"
              strokeWidth="2.5"
            />

            {/* Antenna */}
            <line x1="60" y1="28" x2="60" y2="12" stroke="#818cf8" strokeWidth="3" strokeLinecap="round" />
            <circle cx="60" cy="9" r="6" fill="#38bdf8" className="animate-pulse" />

            {/* Ear Pods */}
            <rect x="12" y="52" width="8" height="24" rx="4" fill="#4338ca" />
            <rect x="100" y="52" width="8" height="24" rx="4" fill="#4338ca" />

            {/* Screen Visor */}
            <rect x="30" y="40" width="60" height="40" rx="14" fill="#090d16" stroke="#312e81" strokeWidth="1.5" />

            {/* Expressive Glowing Eyes */}
            {mascot.emotion === 'excited' || mascot.emotion === 'happy' ? (
              // Happy curved eyes ^^
              <>
                <path
                  d="M40 58 Q47 50 54 58"
                  stroke="#38bdf8"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                  fill="none"
                />
                <path
                  d="M66 58 Q73 50 80 58"
                  stroke="#38bdf8"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                  fill="none"
                />
              </>
            ) : mascot.emotion === 'alert' ? (
              // Alert round eyes
              <>
                <circle cx="47" cy="56" r="6" fill="#fb7185" className="animate-ping" />
                <circle cx="47" cy="56" r="5" fill="#f43f5e" />
                <circle cx="73" cy="56" r="6" fill="#fb7185" className="animate-ping" />
                <circle cx="73" cy="56" r="5" fill="#f43f5e" />
              </>
            ) : (
              // Friendly default oval eyes with sparkle
              <>
                <ellipse cx="47" cy="56" rx="5" ry="7" fill="#38bdf8" />
                <circle cx="49" cy="54" r="2" fill="#ffffff" />
                <ellipse cx="73" cy="56" rx="5" ry="7" fill="#38bdf8" />
                <circle cx="75" cy="54" r="2" fill="#ffffff" />
              </>
            )}

            {/* Cute Smile */}
            <path
              d="M52 68 Q60 74 68 68"
              stroke="#818cf8"
              strokeWidth="2.5"
              strokeLinecap="round"
              fill="none"
            />

            {/* Cheeks blush */}
            <circle cx="37" cy="66" r="3" fill="#f43f5e" opacity="0.6" />
            <circle cx="83" cy="66" r="3" fill="#f43f5e" opacity="0.6" />

            {/* Gradients */}
            <defs>
              <linearGradient id="mascotGrad" x1="20" y1="28" x2="100" y2="100" gradientUnits="userSpaceOnUse">
                <stop stopColor="#1e1b4b" />
                <stop offset="0.5" stopColor="#312e81" />
                <stop offset="1" stopColor="#1e1b4b" />
              </linearGradient>
            </defs>
          </svg>
        </div>

        {/* Tap hint pill */}
        <div className="text-center mt-1">
          <span className="text-[10px] font-medium text-slate-400 group-hover:text-indigo-300 transition-colors bg-slate-900/60 px-2 py-0.5 rounded-full border border-slate-800">
            Нажмите на маскота
          </span>
        </div>
      </div>
    </div>
  );
};
