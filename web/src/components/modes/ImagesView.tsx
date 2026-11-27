import React, { useEffect, useRef, useState } from 'react';
import {
  ArrowLeft,
  Sparkles,
  Wand2,
  Image as ImageIcon,
  FolderOpen,
  Upload,
  Download,
  Trash2,
  RotateCcw,
  Loader2,
  Layers,
  X,
  SlidersHorizontal,
  Play,
  Check,
} from 'lucide-react';
import { imagesApi } from '../../api/images';
import {
  ImageMode,
  ImageModelOption,
  ImagePreset,
  ImageProject,
} from '../../types/images';

interface ImagesViewProps {
  onBack: () => void;
  onSuccessToast: (msg: string) => void;
  onErrorToast: (msg: string) => void;
}

type StudioTab = 'presets' | 'studio' | 'projects';
type PresetFilter = 'all' | 'edit' | 'generate';

const ASPECT_RATIOS = ['1:1', '16:9', '9:16', '4:3'] as const;

const QUICK_REFINE_CHIPS = [
  'Фон темнее',
  'Больше света',
  'Минимализм',
  'Убрать детали',
];

const STATUS_BADGES: Record<string, { label: string; cls: string }> = {
  draft: { label: 'Черновик', cls: 'bg-slate-100 text-slate-600' },
  processing: { label: 'В работе', cls: 'bg-amber-50 text-amber-700' },
  ready: { label: 'Готово', cls: 'bg-emerald-50 text-emerald-700' },
  failed: { label: 'Ошибка', cls: 'bg-rose-50 text-rose-700' },
};

function getErrMessage(err: any, fallback: string): string {
  return err?.response?.data?.error || err?.message || fallback;
}

export const ImagesView: React.FC<ImagesViewProps> = ({
  onBack,
  onSuccessToast,
  onErrorToast,
}) => {
  const [activeTab, setActiveTab] = useState<StudioTab>('presets');
  const [isLoading, setIsLoading] = useState(true);
  const [isBusy, setIsBusy] = useState(false);

  // Catalog & projects
  const [presets, setPresets] = useState<ImagePreset[]>([]);
  const [models, setModels] = useState<ImageModelOption[]>([]);
  const [projects, setProjects] = useState<ImageProject[]>([]);
  const [presetFilter, setPresetFilter] = useState<PresetFilter>('all');
  const [cardBeforeView, setCardBeforeView] = useState<Record<string, boolean>>({});

  // Studio form state
  const [mode, setMode] = useState<ImageMode>('generate');
  const [selectedPresetId, setSelectedPresetId] = useState<string>('');
  const [prompt, setPrompt] = useState('');
  const [aspectRatio, setAspectRatio] = useState<string>('1:1');
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [sourceFile, setSourceFile] = useState<File | null>(null);
  const [sourcePreviewUrl, setSourcePreviewUrl] = useState<string>('');
  const [useExampleSource, setUseExampleSource] = useState(false);
  const [isDragging, setIsDragging] = useState(false);

  // Active project & result state
  const [activeProject, setActiveProject] = useState<ImageProject | null>(null);
  const [showBeforeResult, setShowBeforeResult] = useState(false);
  const [refinePrompt, setRefinePrompt] = useState('');
  const [selectedRevisionVersion, setSelectedRevisionVersion] = useState<number | null>(null);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    loadInitialData();
  }, []);

  useEffect(() => {
    if (!sourceFile) {
      setSourcePreviewUrl('');
      return;
    }
    const url = URL.createObjectURL(sourceFile);
    setSourcePreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [sourceFile]);

  const loadInitialData = async () => {
    try {
      setIsLoading(true);
      const [presetsRes, modelsRes, projectsRes] = await Promise.all([
        imagesApi.getPresets(),
        imagesApi.getModels(),
        imagesApi.listProjects(),
      ]);
      setPresets(presetsRes);
      setModels(modelsRes);
      setProjects(projectsRes);
      if (modelsRes.length > 0 && !selectedModel) {
        setSelectedModel(modelsRes[0].id);
      }
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось загрузить Фотостудию'));
    } finally {
      setIsLoading(false);
    }
  };

  const refreshProjects = async () => {
    try {
      const list = await imagesApi.listProjects();
      setProjects(list);
    } catch (_) {
      // Ignore silent refresh errors
    }
  };

  const applyProjectToState = (proj: ImageProject) => {
    setActiveProject(proj);
    setShowBeforeResult(false);
    setSelectedRevisionVersion(proj.current_version || null);
    setMode(proj.mode);
    setSelectedPresetId(proj.preset_id || '');
    setAspectRatio(proj.aspect_ratio || '1:1');
    if (proj.image_model) {
      setSelectedModel(proj.image_model);
    }
  };

  const handleFileSelected = (file: File | null) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      onErrorToast('Выберите файл изображения (PNG, JPG или WebP)');
      return;
    }
    setSourceFile(file);
    setUseExampleSource(false);
  };

  const handleTryExample = async (preset: ImagePreset) => {
    setSelectedPresetId(preset.id);
    setMode(preset.mode);
    setPrompt(preset.example_prompt);
    setAspectRatio(preset.default_aspect_ratio || '1:1');
    setSourceFile(null);
    const useEx = preset.mode === 'edit';
    setUseExampleSource(useEx);
    setActiveTab('studio');

    try {
      setIsBusy(true);
      const proj = await imagesApi.createProject({
        mode: preset.mode,
        preset_id: preset.id,
        prompt: preset.example_prompt,
        aspect_ratio: preset.default_aspect_ratio || '1:1',
        image_model: selectedModel || undefined,
        use_example_source: useEx,
      });
      applyProjectToState(proj);
      onSuccessToast('Пример сгенерирован!');
      await refreshProjects();
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось запустить пример'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleUseOwnPreset = (preset: ImagePreset) => {
    setSelectedPresetId(preset.id);
    setMode(preset.mode);
    setPrompt(preset.example_prompt);
    setAspectRatio(preset.default_aspect_ratio || '1:1');
    setSourceFile(null);
    setUseExampleSource(false);
    setActiveProject(null);
    setActiveTab('studio');
  };

  const handleRunStudio = async () => {
    const trimmed = prompt.trim();
    if (!trimmed) {
      onErrorToast('Введите текстовый запрос');
      return;
    }
    if (mode === 'edit' && !sourceFile && !useExampleSource) {
      onErrorToast('Загрузите фото для редактирования');
      return;
    }

    try {
      setIsBusy(true);
      const proj = await imagesApi.createProject({
        mode,
        preset_id: selectedPresetId || undefined,
        prompt: trimmed,
        aspect_ratio: aspectRatio,
        image_model: selectedModel || undefined,
        use_example_source: useExampleSource,
        sourceFile: mode === 'edit' ? sourceFile : null,
      });
      applyProjectToState(proj);
      onSuccessToast('Изображение готово!');
      await refreshProjects();
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось выполнить генерацию'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleRefine = async (instruction?: string) => {
    if (!activeProject) return;
    const text = (instruction ?? refinePrompt).trim();
    if (!text) {
      onErrorToast('Опишите, что нужно изменить');
      return;
    }

    try {
      setIsBusy(true);
      const proj = await imagesApi.refineProject(
        activeProject.id,
        text,
        selectedModel || undefined
      );
      applyProjectToState(proj);
      setRefinePrompt('');
      onSuccessToast(`Готова версия v${proj.current_version}`);
      await refreshProjects();
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось применить правку'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleRestoreVersion = async (version: number) => {
    if (!activeProject) return;
    try {
      setIsBusy(true);
      const proj = await imagesApi.restoreProject(activeProject.id, version);
      applyProjectToState(proj);
      onSuccessToast(`Восстановлена версия v${version}`);
      await refreshProjects();
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось восстановить версию'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleOpenProject = async (projectId: number) => {
    try {
      setIsBusy(true);
      const full = await imagesApi.getProject(projectId);
      applyProjectToState(full);
      setSourceFile(null);
      setUseExampleSource(false);
      setActiveTab('studio');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось открыть проект'));
    } finally {
      setIsBusy(false);
    }
  };

  const handleDeleteProject = async (
    e: React.MouseEvent,
    projectId: number
  ) => {
    e.stopPropagation();
    try {
      await imagesApi.deleteProject(projectId);
      setProjects((prev) => prev.filter((p) => p.id !== projectId));
      if (activeProject?.id === projectId) {
        setActiveProject(null);
      }
      onSuccessToast('Работа удалена');
    } catch (err: any) {
      onErrorToast(getErrMessage(err, 'Не удалось удалить работу'));
    }
  };

  const filteredPresets = presets.filter((p) =>
    presetFilter === 'all' ? true : p.mode === presetFilter
  );

  const selectedPreset = presets.find((p) => p.id === selectedPresetId);

  return (
    <div className="w-full max-w-md sm:max-w-2xl mx-auto px-4 py-3 space-y-4">
      {/* Top Bar */}
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-slate-600 hover:text-teal-700 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Назад</span>
        </button>
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-teal-50 text-teal-600 flex items-center justify-center">
            <ImageIcon className="w-4 h-4" />
          </div>
          <h1 className="font-bold text-base text-slate-900">Фотостудия</h1>
        </div>
        <div className="w-12" />
      </div>

      {/* Navigation Tabs */}
      <div className="grid grid-cols-3 gap-1.5 bg-slate-100 p-1 rounded-xl">
        <button
          type="button"
          onClick={() => setActiveTab('presets')}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg text-xs sm:text-sm font-semibold transition-all ${
            activeTab === 'presets'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Sparkles className="w-3.5 h-3.5" />
          <span>Сценарии</span>
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('studio')}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg text-xs sm:text-sm font-semibold transition-all ${
            activeTab === 'studio'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <Wand2 className="w-3.5 h-3.5" />
          <span>Студия</span>
        </button>
        <button
          type="button"
          onClick={() => {
            setActiveTab('projects');
            refreshProjects();
          }}
          className={`flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg text-xs sm:text-sm font-semibold transition-all ${
            activeTab === 'projects'
              ? 'bg-white text-teal-700 shadow-sm'
              : 'text-slate-600 hover:text-slate-900'
          }`}
        >
          <FolderOpen className="w-3.5 h-3.5" />
          <span>Мои работы</span>
        </button>
      </div>

      {isLoading ? (
        <div className="flex flex-col items-center justify-center py-16 text-slate-500 gap-2">
          <Loader2 className="w-6 h-6 animate-spin text-teal-600" />
          <span className="text-sm">Загружаем Фотостудию...</span>
        </div>
      ) : (
        <>
          {/* TAB 1: PRESETS */}
          {activeTab === 'presets' && (
            <div className="space-y-3">
              {/* Category Filter */}
              <div className="flex items-center gap-2">
                {(
                  [
                    { id: 'all', label: 'Все' },
                    { id: 'edit', label: 'Исправление' },
                    { id: 'generate', label: 'Генерация' },
                  ] as const
                ).map((cat) => (
                  <button
                    key={cat.id}
                    type="button"
                    onClick={() => setPresetFilter(cat.id)}
                    className={`px-3 py-1.5 rounded-full text-xs font-semibold transition-colors ${
                      presetFilter === cat.id
                        ? 'bg-teal-600 text-white'
                        : 'bg-white border border-slate-200 text-slate-600 hover:border-teal-300'
                    }`}
                  >
                    {cat.label}
                  </button>
                ))}
              </div>

              {/* Preset Cards */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {filteredPresets.map((preset) => {
                  const hasBefore = Boolean(preset.example_before_url);
                  const showingBefore = Boolean(
                    hasBefore && cardBeforeView[preset.id]
                  );
                  const imgSrc = showingBefore
                    ? preset.example_before_url
                    : preset.example_after_url;

                  return (
                    <div
                      key={preset.id}
                      className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden flex flex-col"
                    >
                      <div className="relative aspect-[4/3] bg-slate-100 overflow-hidden">
                        <img
                          src={imgSrc}
                          alt={preset.title}
                          className="w-full h-full object-cover"
                        />
                        <div className="absolute top-2 left-2 flex items-center gap-1.5">
                          <span className="px-2 py-0.5 rounded-full text-[11px] font-semibold bg-slate-900/75 text-white backdrop-blur-sm">
                            {preset.mode === 'edit' ? 'Редактирование' : 'Генерация'}
                          </span>
                          <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-white/90 text-slate-700">
                            {preset.default_aspect_ratio}
                          </span>
                        </div>
                        {hasBefore && (
                          <button
                            type="button"
                            onClick={() =>
                              setCardBeforeView((prev) => ({
                                ...prev,
                                [preset.id]: !prev[preset.id],
                              }))
                            }
                            className="absolute bottom-2 right-2 px-2.5 py-1 rounded-full text-[11px] font-semibold bg-slate-900/80 text-white hover:bg-slate-900 transition-colors"
                          >
                            {showingBefore ? 'До → Показать После' : 'После → Показать До'}
                          </button>
                        )}
                      </div>

                      <div className="p-3.5 flex-1 flex flex-col justify-between space-y-3">
                        <div className="space-y-1.5">
                          <h3 className="font-bold text-sm text-slate-900">
                            {preset.title}
                          </h3>
                          <p className="text-xs text-slate-500 leading-relaxed">
                            {preset.description}
                          </p>
                          <div className="text-[11px] text-slate-600 bg-slate-50 border border-slate-100 rounded-lg p-2 line-clamp-2">
                            «{preset.example_prompt}»
                          </div>
                        </div>

                        <div className="grid grid-cols-2 gap-2 pt-1">
                          <button
                            type="button"
                            disabled={isBusy}
                            onClick={() => handleTryExample(preset)}
                            className="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl bg-teal-600 hover:bg-teal-700 disabled:opacity-50 text-white text-xs font-semibold transition-colors"
                          >
                            <Play className="w-3.5 h-3.5" />
                            <span>Попробовать пример</span>
                          </button>
                          <button
                            type="button"
                            disabled={isBusy}
                            onClick={() => handleUseOwnPreset(preset)}
                            className="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl bg-slate-100 hover:bg-slate-200 disabled:opacity-50 text-slate-700 text-xs font-semibold transition-colors"
                          >
                            <SlidersHorizontal className="w-3.5 h-3.5" />
                            <span>Со своим фото / запросом</span>
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* TAB 2: STUDIO */}
          {activeTab === 'studio' && (
            <div className="space-y-4">
              {/* Controls Card */}
              <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-4 space-y-4">
                {/* Mode Switch */}
                <div className="flex items-center justify-between gap-2">
                  <div className="grid grid-cols-2 gap-1 bg-slate-100 p-1 rounded-xl flex-1">
                    <button
                      type="button"
                      onClick={() => setMode('generate')}
                      className={`py-1.5 px-3 rounded-lg text-xs font-semibold transition-all ${
                        mode === 'generate'
                          ? 'bg-white text-teal-700 shadow-sm'
                          : 'text-slate-600'
                      }`}
                    >
                      Генерация
                    </button>
                    <button
                      type="button"
                      onClick={() => setMode('edit')}
                      className={`py-1.5 px-3 rounded-lg text-xs font-semibold transition-all ${
                        mode === 'edit'
                          ? 'bg-white text-teal-700 shadow-sm'
                          : 'text-slate-600'
                      }`}
                    >
                      Редактирование
                    </button>
                  </div>

                  {selectedPreset && (
                    <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-teal-50 text-teal-700 text-xs font-medium">
                      <span>{selectedPreset.title}</span>
                      <button
                        type="button"
                        onClick={() => setSelectedPresetId('')}
                        className="hover:text-teal-900"
                        title="Сбросить сценарий"
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </span>
                  )}
                </div>

                {/* Upload Dropzone for Edit Mode */}
                {mode === 'edit' && (
                  <div className="space-y-2">
                    <label className="block text-xs font-semibold text-slate-700">
                      Исходное фото
                    </label>
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      className="hidden"
                      onChange={(e) =>
                        handleFileSelected(e.target.files?.[0] || null)
                      }
                    />
                    {sourcePreviewUrl ? (
                      <div className="relative rounded-xl overflow-hidden border border-slate-200 bg-slate-50">
                        <img
                          src={sourcePreviewUrl}
                          alt="Исходное фото"
                          className="w-full max-h-56 object-contain mx-auto"
                        />
                        <button
                          type="button"
                          onClick={() => {
                            setSourceFile(null);
                            if (fileInputRef.current) fileInputRef.current.value = '';
                          }}
                          className="absolute top-2 right-2 p-1.5 rounded-full bg-slate-900/75 text-white hover:bg-slate-900"
                          title="Удалить фото"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    ) : (
                      <div
                        onClick={() => fileInputRef.current?.click()}
                        onDragOver={(e) => {
                          e.preventDefault();
                          setIsDragging(true);
                        }}
                        onDragLeave={() => setIsDragging(false)}
                        onDrop={(e) => {
                          e.preventDefault();
                          setIsDragging(false);
                          handleFileSelected(e.dataTransfer.files?.[0] || null);
                        }}
                        className={`border-2 border-dashed rounded-xl p-5 text-center cursor-pointer transition-colors ${
                          isDragging
                            ? 'border-teal-500 bg-teal-50/60'
                            : 'border-slate-300 hover:border-teal-400 bg-slate-50/50'
                        }`}
                      >
                        <Upload className="w-6 h-6 text-teal-600 mx-auto mb-1.5" />
                        <p className="text-xs font-semibold text-slate-700">
                          Нажмите или перетащите фото сюда
                        </p>
                        <p className="text-[11px] text-slate-400 mt-0.5">
                          PNG, JPG, WebP до 10 МБ
                        </p>
                        {useExampleSource && (
                          <span className="inline-block mt-2 px-2 py-0.5 rounded-full bg-teal-50 text-teal-700 text-[11px] font-medium">
                            Используется демо-фото сценария
                          </span>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {/* Prompt Textarea */}
                <div className="space-y-1.5">
                  <label className="block text-xs font-semibold text-slate-700">
                    {mode === 'edit' ? 'Что сделать с фото?' : 'Описание изображения'}
                  </label>
                  <textarea
                    rows={3}
                    value={prompt}
                    onChange={(e) => setPrompt(e.target.value)}
                    placeholder={
                      mode === 'edit'
                        ? 'Например: замени фон на светлую студию и добавь мягкие тени'
                        : 'Например: минималистичный постер кофейни в тёплых тонах'
                    }
                    className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500 resize-none"
                  />
                </div>

                {/* Aspect Ratio & Model */}
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="space-y-1.5">
                    <label className="block text-xs font-semibold text-slate-700">
                      Формат
                    </label>
                    <div className="grid grid-cols-4 gap-1.5">
                      {ASPECT_RATIOS.map((ratio) => (
                        <button
                          key={ratio}
                          type="button"
                          onClick={() => setAspectRatio(ratio)}
                          className={`py-1.5 rounded-lg text-xs font-semibold border transition-colors ${
                            aspectRatio === ratio
                              ? 'bg-teal-50 border-teal-500 text-teal-700'
                              : 'bg-white border-slate-200 text-slate-600 hover:border-slate-300'
                          }`}
                        >
                          {ratio}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="space-y-1.5">
                    <label className="block text-xs font-semibold text-slate-700">
                      Модель
                    </label>
                    <select
                      value={selectedModel}
                      onChange={(e) => setSelectedModel(e.target.value)}
                      className="w-full rounded-xl border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-800 focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500 h-[34px]"
                    >
                      {models.length === 0 ? (
                        <option value="">По умолчанию</option>
                      ) : (
                        models.map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.label}
                          </option>
                        ))
                      )}
                    </select>
                  </div>
                </div>

                {/* Primary Run Button */}
                <button
                  type="button"
                  disabled={isBusy}
                  onClick={handleRunStudio}
                  className="w-full inline-flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl bg-teal-600 hover:bg-teal-700 disabled:opacity-60 text-white text-sm font-semibold shadow-sm transition-colors"
                >
                  {isBusy ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Создаём изображение...</span>
                    </>
                  ) : (
                    <>
                      <Sparkles className="w-4 h-4" />
                      <span>
                        {mode === 'edit' ? 'Обработать фото' : 'Сгенерировать'}
                      </span>
                    </>
                  )}
                </button>
              </div>

              {/* Active Project Result Viewer */}
              {activeProject && activeProject.current_image_url && (
                <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-4 space-y-4">
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <h3 className="font-bold text-sm text-slate-900 truncate">
                        {activeProject.title || 'Результат'}
                      </h3>
                      <p className="text-[11px] text-slate-500">
                        Версия v{activeProject.current_version} •{' '}
                        {activeProject.aspect_ratio}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      {activeProject.source_image_url && (
                        <button
                          type="button"
                          onClick={() => setShowBeforeResult((v) => !v)}
                          className={`px-2.5 py-1.5 rounded-lg text-xs font-semibold border transition-colors ${
                            showBeforeResult
                              ? 'bg-slate-900 text-white border-slate-900'
                              : 'bg-slate-50 text-slate-700 border-slate-200 hover:bg-slate-100'
                          }`}
                        >
                          {showBeforeResult ? 'Показать После' : 'До / После'}
                        </button>
                      )}
                      <a
                        href={activeProject.current_image_url}
                        download={`image-v${activeProject.current_version}.png`}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-teal-50 hover:bg-teal-100 text-teal-700 text-xs font-semibold transition-colors"
                      >
                        <Download className="w-3.5 h-3.5" />
                        <span>Скачать</span>
                      </a>
                    </div>
                  </div>

                  {/* Image Preview */}
                  <div className="relative rounded-xl overflow-hidden bg-slate-100 border border-slate-200">
                    <img
                      src={
                        showBeforeResult && activeProject.source_image_url
                          ? activeProject.source_image_url
                          : activeProject.current_image_url
                      }
                      alt={activeProject.title}
                      className="w-full max-h-[420px] object-contain mx-auto"
                    />
                    {activeProject.source_image_url && (
                      <span className="absolute top-2 left-2 px-2 py-0.5 rounded-full text-[11px] font-semibold bg-slate-900/75 text-white">
                        {showBeforeResult ? 'До' : 'После'}
                      </span>
                    )}
                  </div>

                  {/* Quick Iterative Refine */}
                  <div className="space-y-2 pt-1 border-t border-slate-100">
                    <label className="block text-xs font-semibold text-slate-700">
                      Что изменить?
                    </label>
                    <div className="flex flex-wrap gap-1.5">
                      {QUICK_REFINE_CHIPS.map((chip) => (
                        <button
                          key={chip}
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleRefine(chip)}
                          className="px-2.5 py-1 rounded-full bg-slate-100 hover:bg-teal-50 hover:text-teal-700 text-slate-700 text-xs font-medium transition-colors"
                        >
                          {chip}
                        </button>
                      ))}
                    </div>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={refinePrompt}
                        onChange={(e) => setRefinePrompt(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' && !isBusy) {
                            e.preventDefault();
                            handleRefine();
                          }
                        }}
                        placeholder="Например: сделай фон теплее и добавь блик"
                        className="flex-1 rounded-xl border border-slate-200 px-3 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-teal-500/30 focus:border-teal-500"
                      />
                      <button
                        type="button"
                        disabled={isBusy || !refinePrompt.trim()}
                        onClick={() => handleRefine()}
                        className="px-3.5 py-2 rounded-xl bg-slate-900 hover:bg-slate-800 disabled:opacity-50 text-white text-xs font-semibold transition-colors shrink-0"
                      >
                        Применить
                      </button>
                    </div>
                  </div>

                  {/* Revision Strip */}
                  {activeProject.revisions &&
                    activeProject.revisions.length > 0 && (
                      <div className="space-y-2 pt-2 border-t border-slate-100">
                        <div className="flex items-center justify-between">
                          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-700">
                            <Layers className="w-3.5 h-3.5 text-teal-600" />
                            <span>История версий</span>
                          </span>
                          {selectedRevisionVersion !== null &&
                            selectedRevisionVersion !==
                              activeProject.current_version && (
                              <button
                                type="button"
                                disabled={isBusy}
                                onClick={() =>
                                  handleRestoreVersion(selectedRevisionVersion)
                                }
                                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-800 text-xs font-semibold transition-colors"
                              >
                                <RotateCcw className="w-3 h-3" />
                                <span>
                                  Вернуть версию v{selectedRevisionVersion}
                                </span>
                              </button>
                            )}
                        </div>
                        <div className="flex gap-2 overflow-x-auto pb-1">
                          {activeProject.revisions.map((rev) => {
                            const isCurrent =
                              rev.version === activeProject.current_version;
                            const isSelected =
                              rev.version === selectedRevisionVersion;
                            return (
                              <button
                                key={rev.id || rev.version}
                                type="button"
                                onClick={() =>
                                  setSelectedRevisionVersion(rev.version)
                                }
                                title={rev.prompt}
                                className={`flex items-center gap-2 p-1.5 rounded-xl border shrink-0 transition-all ${
                                  isSelected
                                    ? 'border-teal-500 bg-teal-50/70'
                                    : 'border-slate-200 bg-white hover:border-slate-300'
                                }`}
                              >
                                <img
                                  src={rev.image_url}
                                  alt={`v${rev.version}`}
                                  className="w-9 h-9 rounded-lg object-cover bg-slate-100"
                                />
                                <div className="text-left pr-1">
                                  <div className="flex items-center gap-1 text-xs font-bold text-slate-900">
                                    <span>v{rev.version}</span>
                                    {isCurrent && (
                                      <Check className="w-3 h-3 text-teal-600" />
                                    )}
                                  </div>
                                  <div className="text-[10px] text-slate-500 max-w-[90px] truncate">
                                    {rev.action_type}
                                  </div>
                                </div>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                </div>
              )}
            </div>
          )}

          {/* TAB 3: PROJECTS */}
          {activeTab === 'projects' && (
            <div className="space-y-3">
              {projects.length === 0 ? (
                <div className="bg-white rounded-2xl border border-slate-200 p-8 text-center space-y-2">
                  <ImageIcon className="w-8 h-8 text-slate-300 mx-auto" />
                  <p className="text-sm font-semibold text-slate-700">
                    Пока нет сохранённых работ
                  </p>
                  <p className="text-xs text-slate-500">
                    Выберите сценарий или создайте изображение в Студии
                  </p>
                </div>
              ) : (
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  {projects.map((proj) => {
                    const thumb =
                      proj.current_image_url || proj.source_image_url;
                    const badge =
                      STATUS_BADGES[proj.status] || STATUS_BADGES.draft;
                    return (
                      <div
                        key={proj.id}
                        onClick={() => handleOpenProject(proj.id)}
                        className="group bg-white rounded-2xl border border-slate-200 shadow-sm hover:border-teal-400 hover:shadow-md overflow-hidden cursor-pointer transition-all flex flex-col"
                      >
                        <div className="relative aspect-square bg-slate-100 overflow-hidden">
                          {thumb ? (
                            <img
                              src={thumb}
                              alt={proj.title}
                              className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
                            />
                          ) : (
                            <div className="w-full h-full flex items-center justify-center text-slate-300">
                              <ImageIcon className="w-8 h-8" />
                            </div>
                          )}
                          <span className="absolute top-2 left-2 px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-900/75 text-white">
                            v{proj.current_version}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => handleDeleteProject(e, proj.id)}
                            title="Удалить работу"
                            className="absolute top-2 right-2 p-1.5 rounded-full bg-white/90 text-slate-600 hover:text-rose-600 hover:bg-white shadow-sm transition-colors"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                        <div className="p-2.5 space-y-1">
                          <h3 className="text-xs font-bold text-slate-900 line-clamp-1">
                            {proj.title}
                          </h3>
                          <div className="flex items-center justify-between gap-1">
                            <span
                              className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${badge.cls}`}
                            >
                              {badge.label}
                            </span>
                            <span className="text-[10px] text-slate-400">
                              {proj.aspect_ratio}
                            </span>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
};
