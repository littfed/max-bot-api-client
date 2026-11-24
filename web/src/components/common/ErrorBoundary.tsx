import React, { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an unhandled error:', error, errorInfo);
    // Ensure native messenger dismisses loading spinner even if React errored
    try {
      const win = window as any;
      if (win.WebApp?.ready) win.WebApp.ready();
      if (win.Telegram?.WebApp?.ready) win.Telegram.WebApp.ready();
    } catch (_) {}
  }

  handleReload = () => {
    window.location.reload();
  };

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen bg-slate-900 text-white flex flex-col items-center justify-center p-6 text-center select-none">
          <div className="w-16 h-16 rounded-2xl bg-rose-500/20 border border-rose-500/40 flex items-center justify-center mb-4 text-3xl">
            ⚠️
          </div>
          <h2 className="text-lg font-bold text-slate-100 mb-2">Произошла ошибка при загрузке</h2>
          <p className="text-xs text-slate-400 max-w-xs mb-6">
            Приложение встретило непредвиденную проблему. Нажмите кнопку ниже, чтобы перезагрузить.
          </p>
          {this.state.error && (
            <pre className="text-[10px] text-rose-300 bg-black/40 p-3 rounded-xl max-w-xs overflow-x-auto text-left mb-6 font-mono">
              {this.state.error.message || String(this.state.error)}
            </pre>
          )}
          <button
            onClick={this.handleReload}
            className="px-6 py-2.5 rounded-xl bg-teal-500 hover:bg-teal-600 text-white text-xs font-semibold shadow-lg shadow-teal-500/20 transition-all active:scale-95"
          >
            Перезагрузить приложение
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
