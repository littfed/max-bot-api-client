import React, { useEffect, useState } from 'react';
import {
  ArrowLeft,
  Sparkles,
  Plus,
  Image as ImageIcon,
  Copy,
  Check,
  Edit3,
  RefreshCw,
  Wand2,
  History,
  UserCheck,
  Layers,
  ChevronRight,
  Loader2,
  Save,
  X,
  Cpu,
  ChevronDown,
  ChevronUp,
  RotateCcw,
} from 'lucide-react';
import { postsApi } from '../../api/posts';
import {
  BrandProfile,
  ModelsCatalog,
  PostDraft,
  PostHook,
  PostVariant,
} from '../../types/posts';
import { FormattedMessage } from '../common/FormattedMessage';

interface PostsViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

type StudioTab = 'studio' | 'history' | 'profile';
type WizardStep = 1 | 2 | 3 | 4;

interface PlatformMeta {
  id: string;
  label: string;
  maxChars: number;
  badgeClass: string;
}

const PLATFORMS: PlatformMeta[] = [
  { id: 'max', label: 'MAX', maxChars: 4000, badgeClass: 'bg-teal-50 text-teal-700 border-teal-200' },
  { id: 'telegram', label: 'Telegram', maxChars: 4096, badgeClass: 'bg-sky-50 text-sky-700 border-sky-200' },
  { id: 'vk', label: 'VK', maxChars: 15000, badgeClass: 'bg-blue-50 text-blue-700 border-blue-200' },
  { id: 'instagram', label: 'Instagram', maxChars: 2200, badgeClass: 'bg-pink-50 text-pink-700 border-pink-200' },
];

const GOAL_PRESETS = ['Прогрев', 'Продажа', 'Вовлечение', 'Экспертность', 'Анонс'];
const TONE_PRESETS = ['Дружелюбный', 'Экспертный', 'Дерзкий', 'Деловой', 'Сторителлинг'];
const QUICK_REFINE_ACTIONS = [
  { label: 'Короче', instruction: 'Сделай текст короче и динамичнее, сохранив ключевую мысль' },
  { label: 'Экспертнее', instruction: 'Сделай тон более экспертным, добавь аргументации' },
  { label: 'Больше эмодзи', instruction: 'Добавь уместные эмодзи для визуального ритма' },
  { label: 'Сильнее CTA', instruction: 'Усиль призыв к действию в конце поста' },
];

const STATUS_LABELS: Record<string, { label: string; cls: string }> = {
  idea: { label: 'Идея', cls: 'bg-slate-100 text-slate-700' },
  brief: { label: 'Бриф', cls: 'bg-amber-50 text-amber-700' },
  hooks: { label: 'Хуки', cls: 'bg-indigo-50 text-indigo-700' },
  generating: { label: 'Генерация', cls: 'bg-teal-50 text-teal-700' },
  ready: { label: 'Готов', cls: 'bg-emerald-50 text-emerald-700' },
  archived: { label: 'Архив', cls: 'bg-slate-100 text-slate-500' },
};

const TIER_BADGES: Record<string, { label: string; cls: string }> = {
  auto: { label: 'Auto', cls: 'bg-teal-50 text-teal-700 border-teal-200' },
  balanced: { label: 'Balanced', cls: 'bg-sky-50 text-sky-700 border-sky-200' },
  pro: { label: 'Pro', cls: 'bg-violet-50 text-violet-700 border-violet-200' },
  other: { label: 'Other', cls: 'bg-slate-100 text-slate-600 border-slate-200' },
};

const REVISION_SOURCE_LABELS: Record<string, { label: string; cls: string }> = {
  generate: { label: 'Генерация', cls: 'bg-teal-50 text-teal-700' },
  refine: { label: 'ИИ-правка', cls: 'bg-indigo-50 text-indigo-700' },
  manual: { label: 'Вручную', cls: 'bg-amber-50 text-amber-700' },
  restore: { label: 'Откат', cls: 'bg-slate-100 text-slate-700' },
};

function getPlatformMeta(id: string): PlatformMeta {
  const found = PLATFORMS.find((p) => p.id === id.toLowerCase());
  return (
    found || {
      id,
      label: id.toUpperCase(),
      maxChars: 4000,
      badgeClass: 'bg-slate-100 text-slate-700 border-slate-200',
    }
  );
}

function stepFromDraft(draft: PostDraft | null): WizardStep {
  if (!draft) return 1;
  if (draft.status === 'ready' || (draft.variants && draft.variants.length > 0)) return 4;
  if (draft.status === 'hooks' || (draft.hooks && draft.hooks.length > 0)) return 3;
  return 2;
}

function getErrMessage(err: any, fallback: string): string {
  return err?.response?.data?.error || err?.message || fallback;
}

export const PostsView: React.FC<PostsViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeTab, setActiveTab] = useState<StudioTab>('studio');
  const [isLoading, setIsLoading] = useState(true);
  const [isBusy, setIsBusy] = useState(false);

  const [activeDraft, setActiveDraft] = useState<PostDraft | null>(null);
  const [drafts, setDrafts] = useState<PostDraft[]>([]);
  const [wizardStep, setWizardStep] = useState<WizardStep>(1);

  // Step 1: Idea form
  const [rawIdea, setRawIdea] = useState('');
  const [sourceImageUrl, setSourceImageUrl] = useState('');

  // Step 2: Brief form
  const [goal, setGoal] = useState('Вовлечение');
  const [tone, setTone] = useState('Дружелюбный');
  const [targetPlatforms, setTargetPlatforms] = useState<string[]>(['max', 'telegram']);

  // Step 4: Visual & Variant interactions
  const [customImagePrompt, setCustomImagePrompt] = useState('');
  const [isRegeneratingImage, setIsRegeneratingImage] = useState(false);
  const [copiedPlatform, setCopiedPlatform] = useState<string | null>(null);
  const [refiningPlatform, setRefiningPlatform] = useState<string | null>(null);
  const [customInstructions, setCustomInstructions] = useState<Record<string, string>>({});
  const [editingPlatform, setEditingPlatform] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<{ text: string; cta: string; hashtags: string }>({
    text: '',
    cta: '',
    hashtags: '',
  });
  const [activeVariantPlatform, setActiveVariantPlatform] = useState<string>('');
  const [showTextHistory, setShowTextHistory] = useState(false);
  const [restoringKey, setRestoringKey] = useState<string | null>(null);

  // Text model selector
  const [modelsCatalog, setModelsCatalog] = useState<ModelsCatalog | null>(null);
  const [selectedModel, setSelectedModel] = useState<string>('');

  // Brand Profile form
  const [profileName, setProfileName] = useState('Мой бренд');
  const [niche, setNiche] = useState('');
  const [targetAudience, setTargetAudience] = useState('');
  const [toneOfVoice, setToneOfVoice] = useState('');
  const [forbiddenWords, setForbiddenWords] = useState('');
  const [defaultPlatforms, setDefaultPlatforms] = useState<string[]>(['max', 'telegram']);
  const [isSavingProfile, setIsSavingProfile] = useState(false);

  const syncDraftIntoState = (draft: PostDraft) => {
    setActiveDraft(draft);
    setDrafts((prev) => {
      const exists = prev.some((d) => d.id === draft.id);
      return exists ? prev.map((d) => (d.id === draft.id ? draft : d)) : [draft, ...prev];
    });
    setGoal(draft.goal || 'Вовлечение');
    setTone(draft.tone || toneOfVoice || 'Дружелюбный');
    setTargetPlatforms(
      draft.target_platforms && draft.target_platforms.length > 0
        ? draft.target_platforms
        : defaultPlatforms
    );
    if (draft.text_model) {
      setSelectedModel(draft.text_model);
    }
    if (draft.variants && draft.variants.length > 0) {
      setActiveVariantPlatform((prev) =>
        draft.variants.some((v) => v.platform === prev) ? prev : draft.variants[0].platform
      );
    } else {
      setActiveVariantPlatform('');
    }
    setWizardStep(stepFromDraft(draft));
  };

  const applyProfileToForm = (p: BrandProfile | undefined) => {
    if (!p) return;
    setProfileName(p.name || 'Мой бренд');
    setNiche(p.niche || '');
    setTargetAudience(p.target_audience || '');
    setToneOfVoice(p.tone_of_voice || '');
    setForbiddenWords(p.forbidden_words || '');
    setDefaultPlatforms(
      p.default_platforms && p.default_platforms.length > 0 ? p.default_platforms : ['max', 'telegram']
    );
  };

  const loadState = async (draftId?: number) => {
    setIsLoading(true);
    try {
      const state = await postsApi.getState(draftId);
      setDrafts(state.drafts || []);
      const defaultProfile =
        state.profiles?.find((p) => p.is_default) || state.profiles?.[0];
      applyProfileToForm(defaultProfile);

      if (state.active_draft) {
        syncDraftIntoState(state.active_draft);
      } else {
        setActiveDraft(null);
        setWizardStep(1);
        if (defaultProfile?.default_platforms?.length) {
          setTargetPlatforms(defaultProfile.default_platforms);
        }
        if (defaultProfile?.tone_of_voice) {
          setTone(defaultProfile.tone_of_voice);
        }
      }
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось загрузить Контент-студию'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadState();
    postsApi
      .getModels()
      .then((catalog) => {
        setModelsCatalog(catalog);
        setSelectedModel((prev) => prev || catalog.default_model || catalog.curated?.[0]?.id || '');
      })
      .catch(() => {
        // Gracefully ignore /models failure so UI continues to work
      });
  }, []);

  const handleStartNewPost = () => {
    setActiveDraft(null);
    setRawIdea('');
    setSourceImageUrl('');
    setEditingPlatform(null);
    setActiveVariantPlatform('');
    setShowTextHistory(false);
    setSelectedModel(modelsCatalog?.default_model || '');
    setWizardStep(1);
    setActiveTab('studio');
  };

  const handleCreateDraft = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rawIdea.trim() && !sourceImageUrl.trim()) {
      onErrorToast('Опишите идею поста или укажите ссылку на изображение');
      return;
    }
    setIsBusy(true);
    try {
      const draft = await postsApi.createDraft(rawIdea.trim(), sourceImageUrl.trim() || undefined);
      syncDraftIntoState(draft);
      onSuccessToast('Черновик создан');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось создать черновик'));
    } finally {
      setIsBusy(false);
    }
  };

  const togglePlatform = (
    id: string,
    current: string[],
    setter: React.Dispatch<React.SetStateAction<string[]>>
  ) => {
    if (current.includes(id)) {
      if (current.length === 1) {
        onErrorToast('Выберите хотя бы одну платформу');
        return;
      }
      setter(current.filter((p) => p !== id));
    } else {
      setter([...current, id]);
    }
  };

  const handleBriefAndHooks = async () => {
    if (!activeDraft) return;
    setIsBusy(true);
    try {
      const modelArg = selectedModel || undefined;
      await postsApi.updateBrief(
        activeDraft.id,
        goal.trim(),
        tone.trim(),
        targetPlatforms,
        modelArg
      );
      const updated = await postsApi.generateHooks(activeDraft.id, modelArg);
      syncDraftIntoState(updated);
      onSuccessToast('3 хука готовы! Выберите лучший');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось сгенерировать хуки'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleRegenerateHooks = async () => {
    if (!activeDraft) return;
    setIsBusy(true);
    try {
      const updated = await postsApi.generateHooks(activeDraft.id, selectedModel || undefined);
      syncDraftIntoState(updated);
      onSuccessToast('Новые хуки сгенерированы');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось перегенерировать хуки'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleSelectHookAndGenerate = async (hook: PostHook) => {
    if (!activeDraft) return;
    setIsBusy(true);
    try {
      const updated = await postsApi.generatePackage(
        activeDraft.id,
        hook.id,
        selectedModel || undefined
      );
      syncDraftIntoState(updated);
      onSuccessToast('Пакет постов готов!');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось сгенерировать посты'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleCopyVariant = async (variant: PostVariant) => {
    const parts = [variant.text];
    if (variant.cta) parts.push(variant.cta);
    if (variant.hashtags && variant.hashtags.length > 0) {
      parts.push(variant.hashtags.map((h) => (h.startsWith('#') ? h : `#${h}`)).join(' '));
    }
    const fullText = parts.filter(Boolean).join('\n\n');
    try {
      await navigator.clipboard.writeText(fullText);
      setCopiedPlatform(variant.platform);
      onSuccessToast(`Текст для ${getPlatformMeta(variant.platform).label} скопирован`);
      setTimeout(() => setCopiedPlatform(null), 2000);
    } catch {
      onErrorToast('Не удалось скопировать в буфер обмена');
    }
  };

  const handleRefine = async (platform: string, instruction: string) => {
    if (!activeDraft || !instruction.trim()) return;
    setRefiningPlatform(platform);
    try {
      const updated = await postsApi.refineDraft(
        activeDraft.id,
        platform,
        instruction.trim(),
        selectedModel || undefined
      );
      syncDraftIntoState(updated);
      setCustomInstructions((prev) => ({ ...prev, [platform]: '' }));
      onSuccessToast(`Вариант для ${getPlatformMeta(platform).label} обновлён`);
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось улучшить текст'));
    } finally {
      setRefiningPlatform(null);
    }
  };

  const startManualEdit = (variant: PostVariant) => {
    setEditingPlatform(variant.platform);
    setEditForm({
      text: variant.text || '',
      cta: variant.cta || '',
      hashtags: (variant.hashtags || []).join(', '),
    });
  };

  const handleSaveManualEdit = async (platform: string) => {
    if (!activeDraft) return;
    setIsBusy(true);
    try {
      const hashtags = editForm.hashtags
        .split(/[,\s]+/)
        .map((t) => t.trim().replace(/^#/, ''))
        .filter(Boolean);
      const updated = await postsApi.updateVariant(activeDraft.id, platform, {
        platform,
        text: editForm.text,
        cta: editForm.cta.trim() || undefined,
        hashtags: hashtags.length > 0 ? hashtags : undefined,
      });
      syncDraftIntoState(updated);
      setEditingPlatform(null);
      onSuccessToast('Изменения сохранены');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось сохранить изменения'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleRegenerateImage = async () => {
    if (!activeDraft) return;
    setIsRegeneratingImage(true);
    try {
      const updated = await postsApi.regenerateImage(
        activeDraft.id,
        customImagePrompt.trim() || undefined
      );
      syncDraftIntoState(updated);
      setCustomImagePrompt('');
      onSuccessToast('Визуал обновлён');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось обновить визуал'));
    } finally {
      setIsRegeneratingImage(false);
    }
  };

  const handleRestoreImageRevision = async (version: number) => {
    if (!activeDraft) return;
    const key = `image:${version}`;
    setRestoringKey(key);
    try {
      const updated = await postsApi.restoreRevision(activeDraft.id, 'image', version);
      syncDraftIntoState(updated);
      onSuccessToast(`Восстановлен визуал v${version}`);
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось восстановить версию визуала'));
    } finally {
      setRestoringKey(null);
    }
  };

  const handleRestoreTextRevision = async (version: number, platform: string) => {
    if (!activeDraft) return;
    const key = `text:${platform}:${version}`;
    setRestoringKey(key);
    try {
      const updated = await postsApi.restoreRevision(activeDraft.id, 'text', version, platform);
      syncDraftIntoState(updated);
      setEditingPlatform(null);
      onSuccessToast(`Восстановлена версия текста v${version}`);
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось восстановить версию текста'));
    } finally {
      setRestoringKey(null);
    }
  };

  const renderModelSelector = () => {
    if (!modelsCatalog) return null;
    const curated = modelsCatalog.curated || [];
    const allModels = modelsCatalog.all || [];
    if (curated.length === 0 && allModels.length === 0) return null;

    const curatedIds = new Set(curated.map((m) => m.id));
    const extraModels = allModels.filter((id) => id && !curatedIds.has(id));
    const hasCustomSelected =
      Boolean(selectedModel) &&
      !curatedIds.has(selectedModel) &&
      !extraModels.includes(selectedModel);
    const selectedCurated = curated.find((m) => m.id === selectedModel);

    return (
      <div className="space-y-1">
        <label className="inline-flex items-center gap-1 text-xs font-semibold text-slate-700">
          <Cpu className="w-3.5 h-3.5 text-teal-600" />
          Модель текста
        </label>

        <select
          value={selectedModel}
          onChange={(e) => setSelectedModel(e.target.value)}
          className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs text-slate-800 focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
        >
          {curated.length > 0 && (
            <optgroup label="Рекомендуемые">
              {curated.map((opt) => {
                const tierLabel = (TIER_BADGES[opt.tier] || TIER_BADGES.other).label;
                return (
                  <option key={opt.id} value={opt.id}>
                    {opt.label} ({tierLabel})
                  </option>
                );
              })}
            </optgroup>
          )}
          {extraModels.length > 0 && (
            <optgroup label={`Все модели шлюза (${extraModels.length})`}>
              {extraModels.map((id) => (
                <option key={id} value={id}>
                  {id}
                </option>
              ))}
            </optgroup>
          )}
          {hasCustomSelected && (
            <option value={selectedModel}>{selectedModel}</option>
          )}
        </select>

        {selectedCurated?.description && (
          <p className="text-[11px] text-slate-500">{selectedCurated.description}</p>
        )}
      </div>
    );
  };

  const handleOpenDraftFromHistory = async (draftId: number) => {
    setIsBusy(true);
    try {
      const state = await postsApi.getState(draftId);
      if (state.active_draft) {
        syncDraftIntoState(state.active_draft);
        setActiveTab('studio');
      }
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось открыть черновик'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSavingProfile(true);
    try {
      const saved = await postsApi.saveProfile({
        name: profileName.trim() || 'Мой бренд',
        niche: niche.trim(),
        target_audience: targetAudience.trim(),
        tone_of_voice: toneOfVoice.trim(),
        forbidden_words: forbiddenWords.trim(),
        default_platforms: defaultPlatforms,
        is_default: true,
      });
      applyProfileToForm(saved);
      onSuccessToast('Профиль бренда сохранён');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось сохранить профиль'));
    } finally {
      setIsSavingProfile(false);
    }
  };

  const visualUrl = activeDraft?.generated_image_url || activeDraft?.source_image_url;

  return (
    <div className="w-full max-w-md sm:max-w-lg mx-auto px-4 py-3 space-y-4">
      {/* Top Bar */}
      <div className="flex items-center justify-between gap-2">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-slate-600 hover:text-slate-900 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          Назад
        </button>

        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-teal-600" />
          <h1 className="text-base font-bold text-slate-900">Контент-студия</h1>
        </div>

        <button
          onClick={handleStartNewPost}
          className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-teal-600 text-white text-xs font-semibold hover:bg-teal-700 transition-colors shadow-sm"
        >
          <Plus className="w-3.5 h-3.5" />
          Новый
        </button>
      </div>

      {/* 3 Main Tabs */}
      <div className="grid grid-cols-3 gap-1.5 p-1 bg-slate-100 rounded-2xl">
        <button
          onClick={() => setActiveTab('studio')}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-xl text-xs font-semibold transition-all ${
            activeTab === 'studio'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Layers className="w-3.5 h-3.5" />
          Студия
        </button>
        <button
          onClick={() => setActiveTab('history')}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-xl text-xs font-semibold transition-all ${
            activeTab === 'history'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <History className="w-3.5 h-3.5" />
          История ({drafts.length})
        </button>
        <button
          onClick={() => setActiveTab('profile')}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-xl text-xs font-semibold transition-all ${
            activeTab === 'profile'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <UserCheck className="w-3.5 h-3.5" />
          Профиль бренда
        </button>
      </div>

      {isLoading ? (
        <div className="flex flex-col items-center justify-center py-12 text-slate-500">
          <Loader2 className="w-6 h-6 animate-spin text-teal-600 mb-2" />
          <span className="text-sm">Загружаем студию...</span>
        </div>
      ) : (
        <>
          {/* TAB 1: STUDIO */}
          {activeTab === 'studio' && (
            <div className="space-y-4">
              {/* Stepper */}
              <div className="grid grid-cols-4 gap-1.5 bg-white p-2 rounded-2xl border border-slate-200/80 shadow-sm">
                {[
                  { step: 1 as WizardStep, title: '1. Идея' },
                  { step: 2 as WizardStep, title: '2. Бриф' },
                  { step: 3 as WizardStep, title: '3. Хуки' },
                  { step: 4 as WizardStep, title: '4. Пакет' },
                ].map((item) => {
                  const isCurrent = wizardStep === item.step;
                  const canJump =
                    item.step === 1 ||
                    (item.step === 2 && Boolean(activeDraft)) ||
                    (item.step === 3 && Boolean(activeDraft?.hooks?.length)) ||
                    (item.step === 4 && Boolean(activeDraft?.variants?.length));
                  return (
                    <button
                      key={item.step}
                      disabled={!canJump}
                      onClick={() => canJump && setWizardStep(item.step)}
                      className={`py-1.5 px-1 rounded-xl text-[11px] font-semibold transition-all text-center ${
                        isCurrent
                          ? 'bg-teal-600 text-white shadow-sm'
                          : canJump
                          ? 'bg-teal-50 text-teal-700 hover:bg-teal-100'
                          : 'text-slate-400 cursor-not-allowed'
                      }`}
                    >
                      {item.title}
                    </button>
                  );
                })}
              </div>

              {/* STEP 1: IDEA / PHOTO */}
              {wizardStep === 1 && (
                <form
                  onSubmit={handleCreateDraft}
                  className="bg-white rounded-2xl border border-slate-200/90 p-4 shadow-sm space-y-3"
                >
                  <div>
                    <h2 className="text-sm font-bold text-slate-900">Шаг 1. Идея или фото</h2>
                    <p className="text-xs text-slate-500 mt-0.5">
                      Опишите мысль, новость или кейс — ИИ адаптирует пост под ваши площадки.
                    </p>
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1">
                      Сырая идея поста
                    </label>
                    <textarea
                      rows={4}
                      value={rawIdea}
                      onChange={(e) => setRawIdea(e.target.value)}
                      placeholder="Например: Запустили новую линейку кофе в дрипах — удобно брать в дорогу, 5 вкусов..."
                      className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1">
                      Ссылка на фото (необязательно)
                    </label>
                    <div className="relative">
                      <ImageIcon className="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
                      <input
                        type="url"
                        value={sourceImageUrl}
                        onChange={(e) => setSourceImageUrl(e.target.value)}
                        placeholder="https://..."
                        className="w-full rounded-xl border border-slate-200 pl-9 pr-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                      />
                    </div>
                  </div>

                  <button
                    type="submit"
                    disabled={isBusy}
                    className="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-teal-600 text-white text-sm font-semibold hover:bg-teal-700 disabled:opacity-60 transition-colors"
                  >
                    {isBusy ? (
                      <>
                        <Loader2 className="w-4 h-4 animate-spin" />
                        Создаём черновик...
                      </>
                    ) : (
                      <>
                        Перейти к брифу
                        <ChevronRight className="w-4 h-4" />
                      </>
                    )}
                  </button>
                </form>
              )}

              {/* STEP 2: BRIEF */}
              {wizardStep === 2 && activeDraft && (
                <div className="bg-white rounded-2xl border border-slate-200/90 p-4 shadow-sm space-y-4">
                  <div>
                    <h2 className="text-sm font-bold text-slate-900">Шаг 2. Бриф и площадки</h2>
                    <p className="text-xs text-slate-600 mt-1 bg-slate-50 p-2.5 rounded-xl border border-slate-100">
                      {activeDraft.raw_idea}
                    </p>
                    {activeDraft.vision_summary && (
                      <p className="text-xs text-teal-700 mt-1.5 bg-teal-50/70 p-2 rounded-xl">
                        Фото: {activeDraft.vision_summary}
                      </p>
                    )}
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Цель поста
                    </label>
                    <div className="flex flex-wrap gap-1.5 mb-2">
                      {GOAL_PRESETS.map((g) => (
                        <button
                          key={g}
                          type="button"
                          onClick={() => setGoal(g)}
                          className={`px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors ${
                            goal === g
                              ? 'bg-teal-600 text-white border-teal-600'
                              : 'bg-slate-50 text-slate-700 border-slate-200 hover:border-teal-400'
                          }`}
                        >
                          {g}
                        </button>
                      ))}
                    </div>
                    <input
                      type="text"
                      value={goal}
                      onChange={(e) => setGoal(e.target.value)}
                      placeholder="Своя цель..."
                      className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Тон (Tone of Voice)
                    </label>
                    <div className="flex flex-wrap gap-1.5 mb-2">
                      {TONE_PRESETS.map((t) => (
                        <button
                          key={t}
                          type="button"
                          onClick={() => setTone(t)}
                          className={`px-2.5 py-1 rounded-lg text-xs font-medium border transition-colors ${
                            tone === t
                              ? 'bg-teal-600 text-white border-teal-600'
                              : 'bg-slate-50 text-slate-700 border-slate-200 hover:border-teal-400'
                          }`}
                        >
                          {t}
                        </button>
                      ))}
                    </div>
                    <input
                      type="text"
                      value={tone}
                      onChange={(e) => setTone(e.target.value)}
                      placeholder="Свой тон..."
                      className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                    />
                  </div>

                  <div>
                    <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                      Целевые площадки
                    </label>
                    <div className="grid grid-cols-2 gap-2">
                      {PLATFORMS.map((p) => {
                        const selected = targetPlatforms.includes(p.id);
                        return (
                          <button
                            key={p.id}
                            type="button"
                            onClick={() => togglePlatform(p.id, targetPlatforms, setTargetPlatforms)}
                            className={`py-2 px-3 rounded-xl text-xs font-semibold border text-left flex items-center justify-between transition-all ${
                              selected
                                ? 'bg-teal-50 border-teal-500 text-teal-800'
                                : 'bg-white border-slate-200 text-slate-600 hover:border-slate-300'
                            }`}
                          >
                            <span>{p.label}</span>
                            {selected && <Check className="w-3.5 h-3.5 text-teal-600" />}
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  {renderModelSelector()}

                  <button
                    type="button"
                    disabled={isBusy}
                    onClick={handleBriefAndHooks}
                    className="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-teal-600 text-white text-sm font-semibold hover:bg-teal-700 disabled:opacity-60 transition-colors"
                  >
                    {isBusy ? (
                      <>
                        <Loader2 className="w-4 h-4 animate-spin" />
                        Придумываем 3 хука...
                      </>
                    ) : (
                      <>
                        <Wand2 className="w-4 h-4" />
                        Сгенерировать 3 хука
                      </>
                    )}
                  </button>
                </div>
              )}

              {/* STEP 3: HOOKS */}
              {wizardStep === 3 && activeDraft && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <h2 className="text-sm font-bold text-slate-900">Шаг 3. Выберите хук</h2>
                      <p className="text-xs text-slate-500">
                        Нажмите на лучший заход, чтобы сгенерировать посты и визуал
                      </p>
                    </div>
                    <button
                      type="button"
                      disabled={isBusy}
                      onClick={handleRegenerateHooks}
                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-white border border-slate-200 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
                    >
                      <RefreshCw className={`w-3.5 h-3.5 ${isBusy ? 'animate-spin' : ''}`} />
                      Другие
                    </button>
                  </div>

                  {isBusy && (
                    <div className="bg-teal-50 border border-teal-200 rounded-2xl p-3 flex items-center gap-2.5 text-teal-800 text-xs font-medium">
                      <Loader2 className="w-4 h-4 animate-spin shrink-0" />
                      Генерируем пакет постов и визуал для выбранных площадок...
                    </div>
                  )}

                  <div className="space-y-2.5">
                    {(activeDraft.hooks || []).map((hook) => {
                      const isSelected = activeDraft.selected_hook_id === hook.id;
                      return (
                        <button
                          key={hook.id}
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleSelectHookAndGenerate(hook)}
                          className={`w-full text-left p-3.5 rounded-2xl border transition-all bg-white shadow-sm hover:shadow-md disabled:opacity-60 ${
                            isSelected
                              ? 'border-teal-500 ring-1 ring-teal-500/30'
                              : 'border-slate-200/90 hover:border-teal-400'
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2 mb-1">
                            <span className="text-xs font-bold text-slate-900">{hook.title}</span>
                            {hook.angle && (
                              <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-teal-50 text-teal-700">
                                {hook.angle}
                              </span>
                            )}
                          </div>
                          <p className="text-xs text-slate-600 leading-relaxed">{hook.text}</p>
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* STEP 4: GENERATED PACKAGE & VISUAL */}
              {wizardStep === 4 && activeDraft && (
                <div className="space-y-4">
                  {/* Visual Preview Card */}
                  <div className="bg-white rounded-2xl border border-slate-200/90 p-3.5 shadow-sm space-y-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-1.5">
                        <ImageIcon className="w-4 h-4 text-teal-600" />
                        <h3 className="text-xs font-bold text-slate-900">Визуал к посту</h3>
                      </div>
                      <button
                        type="button"
                        disabled={isRegeneratingImage}
                        onClick={handleRegenerateImage}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-slate-100 hover:bg-slate-200 text-xs font-semibold text-slate-700 disabled:opacity-50"
                      >
                        <RefreshCw
                          className={`w-3 h-3 ${isRegeneratingImage ? 'animate-spin' : ''}`}
                        />
                        Обновить
                      </button>
                    </div>

                    {visualUrl ? (
                      <div className="rounded-xl overflow-hidden border border-slate-200 bg-slate-50">
                        <img
                          src={visualUrl}
                          alt="Post visual"
                          className="w-full max-h-60 object-cover"
                        />
                      </div>
                    ) : (
                      <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50 py-6 text-center text-xs text-slate-500">
                        Изображение пока не сгенерировано
                      </div>
                    )}

                    {activeDraft.image_prompt && (
                      <p className="text-[11px] text-slate-500 leading-snug">
                        Промпт: {activeDraft.image_prompt}
                      </p>
                    )}

                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={customImagePrompt}
                        onChange={(e) => setCustomImagePrompt(e.target.value)}
                        placeholder="Промпт для новой картинки (необязательно)..."
                        className="flex-1 rounded-xl border border-slate-200 px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                      />
                      <button
                        type="button"
                        disabled={isRegeneratingImage}
                        onClick={handleRegenerateImage}
                        className="px-3 py-1.5 rounded-xl bg-teal-600 text-white text-xs font-semibold hover:bg-teal-700 disabled:opacity-50"
                      >
                        {isRegeneratingImage ? '...' : 'Создать'}
                      </button>
                    </div>

                    {activeDraft.image_history && activeDraft.image_history.length > 0 && (
                      <div className="pt-2 border-t border-slate-100 space-y-1.5">
                        <div className="flex items-center justify-between">
                          <span className="text-[11px] font-semibold text-slate-600">
                            История визуала ({activeDraft.image_history.length})
                          </span>
                          <span className="text-[10px] text-slate-400">Нажмите для отката</span>
                        </div>
                        <div className="flex gap-2 overflow-x-auto pb-1">
                          {activeDraft.image_history.map((rev) => {
                            const isActiveUrl = rev.url === visualUrl;
                            const isRestoringThis = restoringKey === `image:${rev.version}`;
                            return (
                              <button
                                key={rev.version}
                                type="button"
                                disabled={Boolean(restoringKey)}
                                onClick={() => handleRestoreImageRevision(rev.version)}
                                title={rev.prompt || `Версия v${rev.version}`}
                                className={`relative shrink-0 w-14 h-14 rounded-lg overflow-hidden border transition-all disabled:opacity-60 ${
                                  isActiveUrl
                                    ? 'border-teal-600 ring-2 ring-teal-500/30'
                                    : 'border-slate-200 hover:border-teal-400'
                                }`}
                              >
                                <img
                                  src={rev.url}
                                  alt={`v${rev.version}`}
                                  className="w-full h-full object-cover"
                                />
                                <span
                                  className={`absolute bottom-0.5 right-0.5 px-1 py-0.2 rounded text-[9px] font-bold ${
                                    isActiveUrl
                                      ? 'bg-teal-600 text-white'
                                      : 'bg-slate-900/70 text-white'
                                  }`}
                                >
                                  {isRestoringThis ? '...' : `v${rev.version}`}
                                </span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Platform Tabs + Active Variant Card */}
                  {(() => {
                    const variants = activeDraft.variants || [];
                    if (variants.length === 0) return null;
                    const activeVariant =
                      variants.find((v) => v.platform === activeVariantPlatform) || variants[0];
                    const meta = getPlatformMeta(activeVariant.platform);
                    const charCount = activeVariant.text?.length || 0;
                    const isOverLimit = charCount > meta.maxChars;
                    const isEditing = editingPlatform === activeVariant.platform;
                    const isRefining = refiningPlatform === activeVariant.platform;
                    const isCopied = copiedPlatform === activeVariant.platform;
                    const platformHistory = (activeDraft.text_history || [])
                      .filter(
                        (rev) =>
                          rev.platform.toLowerCase() === activeVariant.platform.toLowerCase()
                      )
                      .sort((a, b) => b.version - a.version);

                    return (
                      <div className="space-y-2.5">
                        {/* Platform Tabs */}
                        <div className="flex gap-1.5 overflow-x-auto pb-0.5">
                          {variants.map((v) => {
                            const pMeta = getPlatformMeta(v.platform);
                            const isTabActive = v.platform === activeVariant.platform;
                            return (
                              <button
                                key={v.platform}
                                type="button"
                                onClick={() => {
                                  setActiveVariantPlatform(v.platform);
                                  if (editingPlatform && editingPlatform !== v.platform) {
                                    setEditingPlatform(null);
                                  }
                                }}
                                className={`shrink-0 px-3 py-1.5 rounded-xl text-xs font-bold border transition-all ${
                                  isTabActive
                                    ? 'bg-teal-600 text-white border-teal-600 shadow-sm'
                                    : 'bg-white text-slate-600 border-slate-200 hover:border-teal-400'
                                }`}
                              >
                                {pMeta.label}
                              </button>
                            );
                          })}
                        </div>

                        {/* Active Variant Card */}
                        <div className="bg-white rounded-2xl border border-slate-200/90 p-3.5 shadow-sm space-y-3">
                          {/* Card Header */}
                          <div className="flex items-center justify-between gap-2">
                            <div className="flex items-center gap-2">
                              <span
                                className={`px-2.5 py-0.5 rounded-lg text-xs font-bold border ${meta.badgeClass}`}
                              >
                                {meta.label}
                              </span>
                              <span
                                className={`text-[11px] font-medium ${
                                  isOverLimit ? 'text-red-600 font-bold' : 'text-slate-400'
                                }`}
                              >
                                {charCount} / {meta.maxChars}
                              </span>
                            </div>

                            <div className="flex items-center gap-1.5">
                              <button
                                type="button"
                                onClick={() =>
                                  isEditing
                                    ? setEditingPlatform(null)
                                    : startManualEdit(activeVariant)
                                }
                                className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium text-slate-600 hover:bg-slate-100"
                              >
                                {isEditing ? (
                                  <>
                                    <X className="w-3.5 h-3.5" />
                                    Отмена
                                  </>
                                ) : (
                                  <>
                                    <Edit3 className="w-3.5 h-3.5" />
                                    Править
                                  </>
                                )}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleCopyVariant(activeVariant)}
                                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-teal-50 text-teal-700 hover:bg-teal-100 text-xs font-semibold"
                              >
                                {isCopied ? (
                                  <>
                                    <Check className="w-3.5 h-3.5" />
                                    Скопировано
                                  </>
                                ) : (
                                  <>
                                    <Copy className="w-3.5 h-3.5" />
                                    Копировать
                                  </>
                                )}
                              </button>
                            </div>
                          </div>

                          {/* Card Body: Edit or Markdown Preview */}
                          {isEditing ? (
                            <div className="space-y-2">
                              <textarea
                                rows={6}
                                value={editForm.text}
                                onChange={(e) =>
                                  setEditForm((prev) => ({ ...prev, text: e.target.value }))
                                }
                                className="w-full rounded-xl border border-slate-200 px-3 py-2 text-xs leading-relaxed focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                              />
                              <input
                                type="text"
                                value={editForm.cta}
                                onChange={(e) =>
                                  setEditForm((prev) => ({ ...prev, cta: e.target.value }))
                                }
                                placeholder="CTA (призыв к действию)"
                                className="w-full rounded-xl border border-slate-200 px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-teal-500/30"
                              />
                              <input
                                type="text"
                                value={editForm.hashtags}
                                onChange={(e) =>
                                  setEditForm((prev) => ({ ...prev, hashtags: e.target.value }))
                                }
                                placeholder="Хэштеги через запятую"
                                className="w-full rounded-xl border border-slate-200 px-3 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-teal-500/30"
                              />
                              <button
                                type="button"
                                disabled={isBusy}
                                onClick={() => handleSaveManualEdit(activeVariant.platform)}
                                className="w-full flex items-center justify-center gap-1.5 py-2 rounded-xl bg-teal-600 text-white text-xs font-semibold hover:bg-teal-700 disabled:opacity-50"
                              >
                                <Save className="w-3.5 h-3.5" />
                                Сохранить правки
                              </button>
                            </div>
                          ) : (
                            <div className="space-y-2">
                              <FormattedMessage
                                content={activeVariant.text}
                                className="text-xs text-slate-800"
                              />
                              {activeVariant.cta && (
                                <p className="text-xs font-semibold text-teal-700 bg-teal-50/60 px-2.5 py-1.5 rounded-lg">
                                  CTA: {activeVariant.cta}
                                </p>
                              )}
                              {activeVariant.hashtags && activeVariant.hashtags.length > 0 && (
                                <div className="flex flex-wrap gap-1">
                                  {activeVariant.hashtags.map((tag, idx) => (
                                    <span
                                      key={idx}
                                      className="text-[11px] text-slate-500 bg-slate-100 px-2 py-0.5 rounded-md"
                                    >
                                      {tag.startsWith('#') ? tag : `#${tag}`}
                                    </span>
                                  ))}
                                </div>
                              )}
                            </div>
                          )}

                          {/* Quick AI Refine, Model Selector & Custom Instruction */}
                          <div className="pt-2.5 border-t border-slate-100 space-y-2.5">
                            {renderModelSelector()}

                            <div className="flex flex-wrap gap-1">
                              {QUICK_REFINE_ACTIONS.map((act) => (
                                <button
                                  key={act.label}
                                  type="button"
                                  disabled={isRefining}
                                  onClick={() =>
                                    handleRefine(activeVariant.platform, act.instruction)
                                  }
                                  className="px-2 py-1 rounded-lg bg-slate-50 hover:bg-teal-50 hover:text-teal-700 border border-slate-200 text-[11px] font-medium text-slate-600 transition-colors disabled:opacity-50"
                                >
                                  {act.label}
                                </button>
                              ))}
                            </div>

                            <div className="flex gap-1.5">
                              <input
                                type="text"
                                value={customInstructions[activeVariant.platform] || ''}
                                onChange={(e) =>
                                  setCustomInstructions((prev) => ({
                                    ...prev,
                                    [activeVariant.platform]: e.target.value,
                                  }))
                                }
                                placeholder="Своя правка для ИИ..."
                                className="flex-1 rounded-xl border border-slate-200 px-2.5 py-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                              />
                              <button
                                type="button"
                                disabled={
                                  isRefining ||
                                  !(customInstructions[activeVariant.platform] || '').trim()
                                }
                                onClick={() =>
                                  handleRefine(
                                    activeVariant.platform,
                                    customInstructions[activeVariant.platform] || ''
                                  )
                                }
                                className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xl bg-slate-900 text-white text-xs font-semibold hover:bg-slate-800 disabled:opacity-50"
                              >
                                {isRefining ? (
                                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                                ) : (
                                  <Wand2 className="w-3.5 h-3.5" />
                                )}
                                ИИ
                              </button>
                            </div>
                          </div>

                          {/* Text Revision History */}
                          {platformHistory.length > 0 && (
                            <div className="pt-2 border-t border-slate-100 space-y-2">
                              <button
                                type="button"
                                onClick={() => setShowTextHistory((prev) => !prev)}
                                className="w-full flex items-center justify-between text-[11px] font-semibold text-slate-600 hover:text-slate-900"
                              >
                                <span className="inline-flex items-center gap-1.5">
                                  <History className="w-3.5 h-3.5 text-teal-600" />
                                  История версий ({platformHistory.length})
                                </span>
                                {showTextHistory ? (
                                  <ChevronUp className="w-3.5 h-3.5" />
                                ) : (
                                  <ChevronDown className="w-3.5 h-3.5" />
                                )}
                              </button>

                              {showTextHistory && (
                                <div className="space-y-1.5 max-h-56 overflow-y-auto pr-0.5">
                                  {platformHistory.map((rev) => {
                                    const srcMeta = REVISION_SOURCE_LABELS[rev.source] || {
                                      label: rev.source,
                                      cls: 'bg-slate-100 text-slate-700',
                                    };
                                    const isRestoringThis =
                                      restoringKey ===
                                      `text:${activeVariant.platform}:${rev.version}`;
                                    return (
                                      <div
                                        key={`${rev.platform}-${rev.version}`}
                                        className="flex items-start justify-between gap-2 p-2 rounded-xl bg-slate-50 border border-slate-200/80"
                                      >
                                        <div className="min-w-0 flex-1 space-y-0.5">
                                          <div className="flex items-center gap-1.5 flex-wrap">
                                            <span className="text-[11px] font-bold text-slate-800">
                                              v{rev.version}
                                            </span>
                                            <span
                                              className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${srcMeta.cls}`}
                                            >
                                              {srcMeta.label}
                                            </span>
                                            {rev.model && (
                                              <span className="text-[10px] font-mono text-slate-500 bg-white px-1.5 py-0.5 rounded border border-slate-200 truncate max-w-[140px]">
                                                {rev.model}
                                              </span>
                                            )}
                                            <span className="text-[10px] text-slate-400">
                                              {new Date(rev.created_at).toLocaleString('ru-RU', {
                                                day: '2-digit',
                                                month: '2-digit',
                                                hour: '2-digit',
                                                minute: '2-digit',
                                              })}
                                            </span>
                                          </div>
                                          {rev.instruction && (
                                            <p className="text-[11px] text-slate-600 italic truncate">
                                              «{rev.instruction}»
                                            </p>
                                          )}
                                        </div>
                                        <button
                                          type="button"
                                          disabled={Boolean(restoringKey)}
                                          onClick={() =>
                                            handleRestoreTextRevision(
                                              rev.version,
                                              activeVariant.platform
                                            )
                                          }
                                          className="shrink-0 inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-white border border-slate-200 hover:border-teal-500 hover:text-teal-700 text-[11px] font-semibold text-slate-700 disabled:opacity-50"
                                        >
                                          <RotateCcw className="w-3 h-3" />
                                          {isRestoringThis ? '...' : 'Вернуть'}
                                        </button>
                                      </div>
                                    );
                                  })}
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })()}
                </div>
              )}
            </div>
          )}

          {/* TAB 2: HISTORY */}
          {activeTab === 'history' && (
            <div className="space-y-2.5">
              {drafts.length === 0 ? (
                <div className="bg-white rounded-2xl border border-slate-200/90 p-8 text-center space-y-2">
                  <Sparkles className="w-8 h-8 text-slate-300 mx-auto" />
                  <p className="text-sm font-semibold text-slate-700">Пока нет черновиков</p>
                  <p className="text-xs text-slate-500">
                    Создайте первый пост во вкладке «Студия»
                  </p>
                </div>
              ) : (
                drafts.map((d) => {
                  const st = STATUS_LABELS[d.status] || {
                    label: d.status,
                    cls: 'bg-slate-100 text-slate-700',
                  };
                  const isCurrent = activeDraft?.id === d.id;
                  return (
                    <div
                      key={d.id}
                      className={`bg-white rounded-2xl border p-3.5 shadow-sm flex items-start justify-between gap-3 ${
                        isCurrent ? 'border-teal-500 ring-1 ring-teal-500/20' : 'border-slate-200/90'
                      }`}
                    >
                      <div className="min-w-0 flex-1 space-y-1.5">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className={`px-2 py-0.5 rounded-md text-[10px] font-bold ${st.cls}`}>
                            {st.label}
                          </span>
                          {(d.target_platforms || []).map((pid) => (
                            <span
                              key={pid}
                              className="text-[10px] font-semibold text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded"
                            >
                              {getPlatformMeta(pid).label}
                            </span>
                          ))}
                        </div>
                        <p className="text-xs font-medium text-slate-900 line-clamp-2">
                          {d.raw_idea || 'Без описания идеи'}
                        </p>
                        <p className="text-[10px] text-slate-400">
                          {new Date(d.updated_at || d.created_at).toLocaleString('ru-RU', {
                            day: '2-digit',
                            month: '2-digit',
                            hour: '2-digit',
                            minute: '2-digit',
                          })}
                        </p>
                      </div>

                      <button
                        type="button"
                        disabled={isBusy}
                        onClick={() => handleOpenDraftFromHistory(d.id)}
                        className="shrink-0 px-3 py-1.5 rounded-xl bg-teal-50 text-teal-700 hover:bg-teal-100 text-xs font-semibold transition-colors"
                      >
                        Открыть
                      </button>
                    </div>
                  );
                })
              )}
            </div>
          )}

          {/* TAB 3: BRAND PROFILE */}
          {activeTab === 'profile' && (
            <form
              onSubmit={handleSaveProfile}
              className="bg-white rounded-2xl border border-slate-200/90 p-4 shadow-sm space-y-3.5"
            >
              <div>
                <h2 className="text-sm font-bold text-slate-900">Профиль бренда</h2>
                <p className="text-xs text-slate-500 mt-0.5">
                  Эти настройки автоматически учитываются ИИ при генерации хуков и постов.
                </p>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  Название бренда / проекта
                </label>
                <input
                  type="text"
                  value={profileName}
                  onChange={(e) => setProfileName(e.target.value)}
                  placeholder="Например: Кофейня Бодрость"
                  className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">Ниша</label>
                <input
                  type="text"
                  value={niche}
                  onChange={(e) => setNiche(e.target.value)}
                  placeholder="Например: Спешелти кофе, онлайн-школа, B2B SaaS..."
                  className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  Целевая аудитория
                </label>
                <textarea
                  rows={2}
                  value={targetAudience}
                  onChange={(e) => setTargetAudience(e.target.value)}
                  placeholder="Кто ваши клиенты, какие у них боли и интересы..."
                  className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  Тон голоса (Tone of Voice)
                </label>
                <input
                  type="text"
                  value={toneOfVoice}
                  onChange={(e) => setToneOfVoice(e.target.value)}
                  placeholder="Например: Дружелюбный, без канцеляризмов, на «ты»"
                  className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  Запрещённые слова и фразы
                </label>
                <input
                  type="text"
                  value={forbiddenWords}
                  onChange={(e) => setForbiddenWords(e.target.value)}
                  placeholder="Через запятую: уникальный, инновационный, скидочка..."
                  className="w-full rounded-xl border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Площадки по умолчанию
                </label>
                <div className="grid grid-cols-2 gap-2">
                  {PLATFORMS.map((p) => {
                    const selected = defaultPlatforms.includes(p.id);
                    return (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() =>
                          togglePlatform(p.id, defaultPlatforms, setDefaultPlatforms)
                        }
                        className={`py-2 px-3 rounded-xl text-xs font-semibold border text-left flex items-center justify-between transition-all ${
                          selected
                            ? 'bg-teal-50 border-teal-500 text-teal-800'
                            : 'bg-white border-slate-200 text-slate-600 hover:border-slate-300'
                        }`}
                      >
                        <span>{p.label}</span>
                        {selected && <Check className="w-3.5 h-3.5 text-teal-600" />}
                      </button>
                    );
                  })}
                </div>
              </div>

              <button
                type="submit"
                disabled={isSavingProfile}
                className="w-full flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-teal-600 text-white text-sm font-semibold hover:bg-teal-700 disabled:opacity-60 transition-colors"
              >
                {isSavingProfile ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Сохранение...
                  </>
                ) : (
                  <>
                    <Save className="w-4 h-4" />
                    Сохранить профиль
                  </>
                )}
              </button>
            </form>
          )}
        </>
      )}
    </div>
  );
};
