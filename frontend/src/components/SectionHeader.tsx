interface SectionHeaderProps {
  title: string;
  tone?: 'sync' | 'node' | 'http' | 'app' | 'db' | 'info';
}

export function SectionHeader({ title, tone = 'sync' }: SectionHeaderProps) {
  return <h2 className={`section-header tone-${tone}`}>{title}</h2>;
}
