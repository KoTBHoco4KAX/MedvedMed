import { greeting } from '../lib/format'

type HeaderProps = {
  firstName: string
  historyOpen: boolean
  onToggleHistory: () => void
  onOpenGuide: () => void
}

export function Header({ firstName, historyOpen, onToggleHistory, onOpenGuide }: HeaderProps) {
  return (
    <header className="shrink-0 bg-white">
      <div className="flex items-center justify-between gap-2 px-4 py-3">
        <div className="min-w-0 flex-1">
          <p className="truncate text-[15px] leading-tight font-semibold text-ink">Цифровой помощник</p>
          <p className="mt-0.5 truncate text-xs text-muted">{greeting(firstName)}</p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <button
            type="button"
            onClick={onOpenGuide}
            className="h-9 whitespace-nowrap rounded-xl bg-white px-2 text-[12px] font-medium text-red-700 transition-colors hover:bg-soft"
          >
            ⚠️ Опасные
          </button>
          <button
            type="button"
            onClick={onToggleHistory}
            className="h-9 whitespace-nowrap rounded-xl bg-brand px-3 text-[13px] font-medium text-white transition-colors hover:bg-brand-hover"
          >
            {historyOpen ? 'Назад' : 'История'}
          </button>
        </div>
      </div>
    </header>
  )
}

type ProgressBarProps = {
  current: number
  total: number
  canGoBack: boolean
  backDisabled?: boolean
  onBack: () => void
}

export function ProgressBar({ current, total, canGoBack, backDisabled = false, onBack }: ProgressBarProps) {
  const safeTotal = total > 0 ? total : 1
  const percent = Math.min(100, Math.max(0, Math.round((current / safeTotal) * 100)))

  return (
    <div className="shrink-0 bg-white px-4 pt-2.5 pb-2">
      <div className="flex items-center gap-2.5">
        <div className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-line" aria-hidden>
          <div
            className="h-full rounded-full bg-brand transition-[width] duration-300 ease-out"
            style={{ width: `${percent}%` }}
          />
        </div>
        <span className="shrink-0 text-[11px] font-medium text-muted">
          Шаг {current} из {total}
        </span>
        {canGoBack && (
          <button
            type="button"
            onClick={onBack}
            disabled={backDisabled}
            className="h-7 shrink-0 rounded-lg border border-[#D1D5DB] bg-white px-2.5 text-[12px] font-medium text-ink transition-colors hover:bg-soft disabled:opacity-50"
          >
            ← Назад
          </button>
        )}
      </div>
    </div>
  )
}
