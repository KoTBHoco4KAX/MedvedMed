type EmergencyCardProps = {
  reason: string
  instruction: string
  onRestart: () => void
}

export function EmergencyCard({ reason, instruction, onRestart }: EmergencyCardProps) {
  return (
    <article data-card="emergency" className="rounded-2xl border-2 border-red-600 bg-white p-5" role="alert">
      <h2 className="text-lg leading-snug font-semibold text-red-700">🚨 Требуется срочная медицинская помощь</h2>

      <p className="mt-3 text-[14px] leading-relaxed text-ink">{reason}</p>
      <p className="mt-3 text-[14px] leading-relaxed font-semibold text-ink">{instruction}</p>
      <p className="mt-2 text-[13px] leading-relaxed text-muted">
        Не ждите окончания опроса — помощь нужна сейчас. Не оставайтесь один и не занимайтесь самолечением.
      </p>

      <div className="mt-4 grid grid-cols-2 gap-2">
        <a
          href="tel:103"
          className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
        >
          Позвонить 103
        </a>
        <a
          href="tel:112"
          className="flex h-11 items-center justify-center rounded-xl bg-red-600 text-[15px] font-semibold text-white transition-colors hover:bg-red-700"
        >
          Позвонить 112
        </a>
      </div>

      <button
        type="button"
        onClick={onRestart}
        className="mt-2 h-11 w-full rounded-xl border border-[#D1D5DB] bg-white text-[15px] font-medium text-ink transition-colors hover:bg-soft"
      >
        Начать заново
      </button>
    </article>
  )
}
