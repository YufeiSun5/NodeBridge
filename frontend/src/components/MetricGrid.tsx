interface Metric {
  label: string;
  value: string | number;
  tone?: 'ok' | 'warn' | 'error' | 'muted';
}

interface MetricGridProps {
  items: Metric[];
}

export function MetricGrid({ items }: MetricGridProps) {
  return (
    <div className="metric-grid">
      {items.map((item) => (
        <div className="metric-card" key={item.label}>
          <span className="metric-label">{item.label}</span>
          <strong className={`metric-value ${item.tone || 'muted'}`}>{item.value}</strong>
        </div>
      ))}
    </div>
  );
}
