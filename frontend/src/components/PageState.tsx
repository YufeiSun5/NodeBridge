interface PageStateProps {
  title: string;
  detail?: string;
}

export function LoadingState({ title = 'Loading' }: Partial<PageStateProps>) {
  return <div className="state-line state-loading">{title}</div>;
}

export function EmptyState({ title, detail }: PageStateProps) {
  return (
    <div className="empty-state">
      <strong>{title}</strong>
      {detail ? <span>{detail}</span> : null}
    </div>
  );
}

export function ErrorState({ title, detail }: PageStateProps) {
  return (
    <div className="empty-state error-state">
      <strong>{title}</strong>
      {detail && detail !== title ? <span>{detail}</span> : null}
    </div>
  );
}

export function InlineErrorState({ title, detail }: PageStateProps) {
  return (
    <div className="inline-error-state">
      <strong>{title}</strong>
      {detail && detail !== title ? <span>{detail}</span> : null}
    </div>
  );
}
