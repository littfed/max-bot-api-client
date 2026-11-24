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
  const getMode = (id: string): ModeItem => {
    return (
      modes.find((m) => m.id === id) || {
        id,
        title: id,
        is_active: false,
      }
    );
  };

  const shopMode = getMode('shop');
  const receptionMode = getMode('reception');
  const remindersMode = getMode('reminders');
  const channelsMode = getMode('channels');
  const giveawaysMode = getMode('giveaways');
  const supportMode = getMode('support');
  const docsMode = getMode('docs');
  const chatMode = getMode('chat');

  return (
    <div className="w-full max-w-2xl mx-auto px-4 py-4 space-y-4">
      {/* Intro Header */}
      <div className="text-center mb-2">
        <h2 className="text-lg sm:text-xl font-bold text-white tracking-tight">
          Виртуальные боты Bot Hub
        </h2>
        <p className="text-xs sm:text-sm text-slate-400">
          Симметричный хаб 8-в-1: выберите режим или нажмите на маскота
        </p>
      </div>

      {/* Row 1: Top 3 Cards (Shop, Reception, Reminders) */}
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3.5">
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
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3.5 items-center">
        <ModeCard
          mode={channelsMode}
          isActiveMode={activeModeId === 'channels'}
          onSelect={onSelectMode}
          onMaintenanceClick={onMaintenanceClick}
        />

        {/* Center Cell: Mascot Renderer */}
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
      <div className="grid grid-cols-3 gap-2.5 sm:gap-3.5">
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
