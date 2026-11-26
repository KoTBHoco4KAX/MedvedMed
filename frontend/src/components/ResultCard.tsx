import { useState } from 'react'
import type { FinalCard } from '../types'
import { urgencyEmoji, urgencyLabel, urgencyTextClass, urgencyTone } from '../lib/format'

type ResultCardProps = {
  card: FinalCard
  onRestart: () => void
}

function summaryText(card: FinalCard): string {
  return [
    'Рекомендация МедВед',
    '',
    `Врач: ${card.specialty}`,
    `Срочность: ${urgencyLabel(card.urgency)}`,
    `Куда обратиться: ${card.place}`,
    '',
    'Обоснование и анамнез:',
    card.reasoning,
    '',
    card.preparationTips,
  ].join('\n')
}

function legacyCopy(text: string): boolean {
  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.top = '-1000px'
  area.style.opacity = '0'
  document.body.appendChild(area)
  area.focus()
  area.select()
  try {
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    area.remove()
  }
}

export function ResultCard({ card, onRestart }: ResultCardProps) {
  const [notice, setNotice] = useState('')
  const tone = urgencyTone(card.urgency)
  const emergency = tone === 'emergency'

  const copySummary = () => {
    const text = summaryText(card)
    setNotice('Сводка скопирована в буфер — откройте чат с врачом в MAX и вставьте')
    const legacyCopied = legacyCopy(text)

    if (navigator.clipboard !== undefined) {
      void navigator.clipboard.writeText(text).catch(() => {
        if (!legacyCopied) legacyCopy(text)
      })
    }
  }

  const downloadMemo = () => {
    const blob = new Blob([summaryText(card)], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'MedVed_pamyatka.txt'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  return (
    <article
      data-card="result"
      className={`rounded-2xl bg-white p-5 ${emergency ? 'border-2 border-red-600' : ''}`}
    >
      {emergency ? (
        <h2 className="text-lg leading-snug font-semibold text-red-700">
          Требуется экстренная медицинская помощь — вызовите скорую (103/112)
        </h2>
      ) : (
        <span className={`inline-flex items-center gap-1.5 text-[12px] font-medium ${urgencyTextClass(tone)}`}>
          <span aria-hidden>{urgencyEmoji(tone)}</span>
          {urgencyLabel(card.urgency)}
        </span>
      )}

      <div className="mt-4">
        <p className="text-[11px] font-medium tracking-wide text-muted uppercase">Врач</p>
        <h2 className="mt-1 text-xl leading-snug font-semibold text-ink">{card.specialty}</h2>
      </div>

      <div className="mt-3">
        <p className="text-[11px] font-medium tracking-wide text-muted uppercase">Куда обратиться</p>
        <p className="mt-1 text-[15px] leading-snug font-medium text-ink">{card.place}</p>
      </div>

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="Обоснование" text={card.reasoning} />

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="По оплате (ОМС/ДМС/платно)" text={card.paymentNote} />

      <div className="my-4 h-px bg-line" aria-hidden />
      <Section title="Что делать и что подготовить" text={card.preparationTips} />

      <p className="mt-5 text-[13px] leading-relaxed font-medium text-amber-800">
        ⚠️ При ухудшении состояния немедленно вызывайте скорую помощь (103/112)
      </p>

      {emergency && (
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
      )}

      <div className="mt-4 grid grid-cols-2 gap-2">
        <button
          type="button"
          onClick={copySummary}
          className="flex min-h-11 items-center justify-center rounded-xl bg-brand px-2 py-2 text-[13px] font-medium text-white transition-colors hover:bg-brand-hover"
        >
          Скопировать сводку
        </button>
        <button
          type="button"
          onClick={downloadMemo}
          className="flex min-h-11 items-center justify-center rounded-xl border border-[#D1D5DB] bg-white px-2 py-2 text-[13px] font-medium text-ink transition-colors hover:bg-soft"
        >
          Скачать памятку
        </button>
      </div>

      {notice !== '' && (
        <p
          className="mt-3 rounded-xl bg-emerald-50 px-3 py-2 text-[13px] leading-relaxed font-medium text-emerald-800"
          role="status"
          aria-live="polite"
        >
          {notice}
        </p>
      )}

      <button
        type="button"
        onClick={onRestart}
        className="mt-3 h-11 w-full rounded-xl bg-brand text-[15px] font-medium text-white transition-colors hover:bg-brand-hover"
      >
        Пройти опрос заново
      </button>
    </article>
  )
}

function Section({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <h3 className="text-[13px] font-semibold text-ink">{title}</h3>
      <p className="mt-1.5 whitespace-pre-line text-[14px] leading-relaxed text-[#374151]">{text}</p>
    </section>
  )
}
