import React, { useState } from 'react';
import { MascotState } from '../../types';

interface MascotRendererProps {
  mascot: MascotState;
  onTap: () => void;
}

export const MascotRenderer: React.FC<MascotRendererProps> = ({ mascot, onTap }) => {
  const [isTapped, setIsTapped] = useState(false);

  const handleTap = () => {
    setIsTapped(true);
    setTimeout(() => setIsTapped(false), 300);

    // Haptic feedback if in Telegram Mini App
    if (window.Telegram?.WebApp?.initData) {
      try {
        (window.Telegram.WebApp as any)?.HapticFeedback?.impactOccurred?.('light');
      } catch (_) {}
    }

    onTap();
  };

  return (
    <div className="flex flex-col items-center justify-center select-none w-full h-full">
      {/* Mascot character with floating animation and tap bounce */}
      <div
        onClick={handleTap}
        className={`relative cursor-pointer group transition-transform duration-150 flex flex-col items-center ${
          isTapped ? 'scale-90' : 'hover:scale-105 active:scale-95'
        }`}
        title="Нажмите на маскота!"
      >
        {/* Soft radial glow behind mascot */}
        <div className="absolute top-1 w-20 h-20 bg-teal-300/30 rounded-full blur-xl pointer-events-none group-hover:bg-teal-400/40 transition-all" />

        {/* Mascot Robot SVG */}
        <div className="relative animate-float-slow">
          <svg
            className="w-16 h-16 sm:w-20 sm:h-20 drop-shadow-md"
            viewBox="0 0 120 120"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            {/* Sparkles around */}
            <circle cx="18" cy="40" r="1.5" fill="#14b8a6" opacity="0.8" />
            <circle cx="102" cy="36" r="2" fill="#14b8a6" opacity="0.8" />
            <path d="M100 48 L102 52 L106 54 L102 56 L100 60 L98 56 L94 54 L98 52 Z" fill="#2dd4bf" opacity="0.6" />

            {/* Antenna Left & Right Ear Pods */}
            <rect x="14" y="50" width="8" height="20" rx="4" fill="#0d9488" stroke="#115e59" strokeWidth="1.5" />
            <rect x="98" y="50" width="8" height="20" rx="4" fill="#0d9488" stroke="#115e59" strokeWidth="1.5" />
            <circle cx="18" cy="46" r="3" fill="#2dd4bf" />
            <circle cx="102" cy="46" r="3" fill="#2dd4bf" />

            {/* Robot Head (White with teal stroke) */}
            <rect
              x="22"
              y="26"
              width="76"
              height="68"
              rx="24"
              fill="#ffffff"
              stroke="#0d9488"
              strokeWidth="2.5"
            />

            {/* Visor Screen (Dark Navy) */}
            <rect
              x="30"
              y="36"
              width="60"
              height="40"
              rx="14"
              fill="#0b1329"
            />

            {/* Expressive Glowing Eyes & Smile on Visor */}
            {mascot.emotion === 'excited' || mascot.emotion === 'happy' ? (
              // Happy curved eyes ^^
              <>
                <path
                  d="M40 54 Q47 46 54 54"
                  stroke="#2dd4bf"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                  fill="none"
                />
                <path
                  d="M66 54 Q73 46 80 54"
                  stroke="#2dd4bf"
                  strokeWidth="3.5"
                  strokeLinecap="round"
                  fill="none"
                />
              </>
            ) : (
              // Friendly default eyes with highlight
              <>
                <ellipse cx="46" cy="53" rx="5" ry="7" fill="#2dd4bf" />
                <circle cx="48" cy="51" r="2" fill="#ffffff" />
                <ellipse cx="74" cy="53" rx="5" ry="7" fill="#2dd4bf" />
                <circle cx="76" cy="51" r="2" fill="#ffffff" />
              </>
            )}

            {/* Visor Smile */}
            <path
              d="M52 64 Q60 70 68 64"
              stroke="#2dd4bf"
              strokeWidth="2.5"
              strokeLinecap="round"
              fill="none"
            />

            {/* Robot Neck & Torso */}
            <rect x="50" y="93" width="20" height="6" rx="2" fill="#0d9488" />
            <path
              d="M36 98 C36 96 46 95 60 95 C74 95 84 96 84 98 L80 114 C80 116 70 118 60 118 C50 118 40 116 40 114 Z"
              fill="#ffffff"
              stroke="#0d9488"
              strokeWidth="2"
            />

            {/* Teal Chest Detail */}
            <rect x="52" y="101" width="16" height="8" rx="3" fill="#ccfbf1" stroke="#0d9488" strokeWidth="1.5" />
            <circle cx="60" cy="105" r="2" fill="#0d9488" />

            {/* Cute Little Robot Arms */}
            <path d="M36 99 C30 102 26 109 30 114 C32 116 36 114 38 108" fill="#ffffff" stroke="#0d9488" strokeWidth="2" strokeLinecap="round" />
            <path d="M84 99 C90 102 94 109 90 114 C88 116 84 114 82 108" fill="#ffffff" stroke="#0d9488" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </div>
      </div>
    </div>
  );
};
