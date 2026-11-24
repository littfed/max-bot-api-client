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

  const shopMode = getMode('shop', 'Магазин');
  const receptionMode = getMode('reception', 'Ресепшен');
  const remindersMode = getMode('reminders', 'Напоминания');
  const channelsMode = getMode('channels', 'Каналы');
  const giveawaysMode = getMode('giveaways', 'Конкурсы');
  const supportMode = getMode('support', 'Помощь');
  const docsMode = getMode('docs', 'Документы');
  const chatMode = getMode('chat', 'Общение');

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 sm:py-4 space-y-3 sm:space-y-4">
      {/* Row 1: Top 3 Cards (Shop, Reception, Reminders) */}
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3">
        <ModeCard
          mode={shopMode}
          isActiveMode={activeModeId === 'shop'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
        <ModeCard
          mode={receptionMode}
          isActiveMode={activeModeId === 'reception'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
        <ModeCard
          mode={remindersMode}
          isActiveMode={activeModeId === 'reminders'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
      </div>

      {/* Row 2: Middle 3 Cells (Channels, Mascot in Center, Giveaways) */}
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3 items-center">
        <ModeCard
          mode={channelsMode}
          isActiveMode={activeModeId === 'channels'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Center Cell: Mascot Renderer with Speech Bubble */}
        <div className="flex justify-center items-center">
          <MascotRenderer mascot={mascot} onTap={onTapMascot} />
        </div>

        <ModeCard
          mode={giveawaysMode}
          isActiveMode={activeModeId === 'giveaways'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
      </div>

      {/* Row 3: Bottom 3 Cards (Support, Documents, Chat) */}
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3">
        <ModeCard
          mode={supportMode}
          isActiveMode={activeModeId === 'support'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
        <ModeCard
          mode={docsMode}
          isActiveMode={activeModeId === 'docs'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
        <ModeCard
          mode={chatMode}
          isActiveMode={activeModeId === 'chat'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />
      </div>
    </div>
  );
};
