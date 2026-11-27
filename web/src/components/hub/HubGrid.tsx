import React from 'react';
import { ModeCard } from './ModeCard';
import { MascotRenderer } from './MascotRenderer';
import { ModeItem, MascotState } from '../../types';

interface HubGridProps {
  modes: ModeItem[];
  activeModeId: string;
  mascot: MascotState;
  onSelectMode: (mode: ModeItem) => void;
  onMaintenanceClick: (mode: ModeItem) => void;
  onTapMascot: () => void;
}

export const HubGrid: React.FC<HubGridProps> = ({
  modes,
  activeModeId,
  mascot,
  onSelectMode,
  onMaintenanceClick,
  onTapMascot,
}) => {
  // Helper to find mode by ID
  const getMode = (id: string, defaultTitle: string): ModeItem => {
    return (
      modes.find((m) => m.id === id) || {
        id,
        title: defaultTitle,
        is_active: false,
      }
    );
  };

  const chatMode = getMode('chat', 'Общение');
  const shopMode = getMode('shop', 'Магазин');
  const receptionMode = getMode('reception', 'Ресепшен');
  const remindersMode = getMode('reminders', 'Напоминания');
  const postsMode = getMode('posts', 'Контент-студия');
  const giveawaysMode = getMode('giveaways', 'Конкурсы');
  const supportMode = getMode('support', 'Помощь');
  const docsMode = getMode('docs', 'Документы');

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4">
      {/* Symmetrical 3x3 Grid: 'Общение' at top-left, Mascot in center */}
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3 items-stretch">
        {/* Cell 1 (Row 1, Col 1): Chat ('Общение') */}
        <ModeCard
          mode={chatMode}
          isActiveMode={activeModeId === 'chat'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 2 (Row 1, Col 2): Shop ('Магазин') */}
        <ModeCard
          mode={shopMode}
          isActiveMode={activeModeId === 'shop'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 3 (Row 1, Col 3): Reception ('Ресепшен') */}
        <ModeCard
          mode={receptionMode}
          isActiveMode={activeModeId === 'reception'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 4 (Row 2, Col 1): Reminders ('Напоминания') */}
        <ModeCard
          mode={remindersMode}
          isActiveMode={activeModeId === 'reminders'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 5 (Row 2, Col 2): Center Mascot */}
        <div className="flex flex-col items-center justify-center p-2 rounded-2xl bg-white border border-slate-200/90 shadow-sm min-h-[96px] sm:min-h-[108px] overflow-hidden">
          <MascotRenderer mascot={mascot} onTap={onTapMascot} />
        </div>

        {/* Cell 6 (Row 2, Col 3): Content Studio ('Контент-студия') */}
        <ModeCard
          mode={postsMode}
          isActiveMode={activeModeId === 'posts'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 7 (Row 3, Col 1): Giveaways ('Конкурсы') */}
        <ModeCard
          mode={giveawaysMode}
          isActiveMode={activeModeId === 'giveaways'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 8 (Row 3, Col 2): Support ('Помощь') */}
        <ModeCard
          mode={supportMode}
          isActiveMode={activeModeId === 'support'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Cell 9 (Row 3, Col 3): Documents ('Документы') */}
        <ModeCard
          mode={docsMode}
          isActiveMode={activeModeId === 'docs'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
      </div>
    </div>
  );
};
