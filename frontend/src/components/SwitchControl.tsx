type SwitchControlProps = {
  checked: boolean;
  label: string;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
};

export function SwitchControl({ checked, label, disabled = false, onChange }: SwitchControlProps) {
  return (
    <button
      className={checked ? 'switch-control active' : 'switch-control'}
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
    >
      <span className="switch-track">
        <span className="switch-thumb" />
      </span>
      <span>{label}</span>
    </button>
  );
}
