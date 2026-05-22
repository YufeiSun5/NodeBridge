import { Component, type ErrorInfo, type ReactNode } from 'react';

type ErrorBoundaryProps = {
  children: ReactNode;
};

type ErrorBoundaryState = {
  error: Error | null;
};

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('NodeBridge UI render error', error, info);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <main className="fatal-error">
        <h1>界面加载失败 / UI Load Failed / UI 読み込み失敗</h1>
        <p>{this.state.error.message}</p>
        <button type="button" onClick={() => window.location.reload()}>
          重新加载 / Reload / 再読み込み
        </button>
      </main>
    );
  }
}
