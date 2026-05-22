type ConfirmDialogProps = {
  title: string;
  detail: string;
  confirmLabel: string;
  cancelLabel: string;
  tone?: 'danger' | 'primary';
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({
  title,
  detail,
  confirmLabel,
  cancelLabel,
  tone = 'danger',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title">
        <h2 id="confirm-dialog-title">{title}</h2>
        <p>{detail}</p>
        <div className="toolbar-row">
          <button className={tone === 'danger' ? 'button-danger' : 'button-primary'} type="button" onClick={onConfirm}>
            {confirmLabel}
          </button>
          <button className="button-secondary" type="button" onClick={onCancel}>
            {cancelLabel}
          </button>
        </div>
      </section>
    </div>
  );
}
