import { useState } from 'react'
import type { HistoryItem } from '../types'
import { formatDateTime, urgencyLabel, urgencyTextClass, urgencyTone } from '../lib/format'

type HistoryPanelProps = {
  items: HistoryItem[] | null
  error: string
  onRetry: () => void
}

export function HistoryPanel({ items, error, onRetry }: HistoryPanelProps) {
  const [openId, setOpenId] = useState<number | null>(null)

  return (
    <div className="flex-1 overflow-y-auto px-4 py-4">
      <h1 className="text-lg font-semibold text-ink">🗂️ История обращений</h1>
      <p className="mt-1 text-[13px] text-muted">Нажмите на карточку, чтобы увидеть рекомендации и ваши ответы</p>

      {items === null && error === '' && (
        <div className="mt-4 flex flex-col gap-3" role="status" aria-live="polite">
          <span className="sr-only">Загружаем историю</span>
          {[0, 1, 2].map((index) => (
            <div key={index} className="h-24 rounded-xl bg-white" aria-hidden />
          ))}
        </div>
      )}

      {error !== '' && (
        <div className="mt-4 rounded-xl bg-white p-4 text-[13px] leading-relaxed text-red-700">
          <p>{error}</p>
          <button type="button" onClick={onRetry} className="mt-2 font-medium underline underline-offset-2">
            Повторить
          </button>
        </div>
      )}

      {items !== null && items.length === 0 && error === '' && (
        <div className="mt-4 rounded-xl bg-white p-6 text-center">
          <p className="text-[15px] font-medium text-ink">Пока нет обращений</p>
          <p className="mt-1 text-[13px] leading-relaxed text-muted">
            Пройдите опрос — карточка с рекомендациями сохранится здесь.
          </p>
        </div>
      )}

      {items !== null && items.length > 0 && (
        <ul className="mt-4 flex flex-col gap-3">
          {items.map((item) => {
            const tone = urgencyTone(item.card.urgency)
            const open = openId === item.id
            // Старые записи могли сохраниться без ответов — не падаем на них.
            const answers = item.answers ?? []

            return (
              <li key={item.id}>
                <article className="overflow-hidden rounded-xl bg-white">
                  <button
                    type="button"
                    onClick={() => setOpenId(open ? null : item.id)}
                    aria-expanded={open}
                    className="w-full px-4 py-3 text-left"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span
                        className={`inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium ${urgencyTextClass(tone)}`}
                      >
                        {urgencyLabel(item.card.urgency)}
                      </span>
                      <span className="flex min-w-0 items-center gap-2 text-[11px] text-muted">
                        <time dateTime={item.createdAt} className="truncate">
                          {formatDateTime(item.createdAt)}
                        </time>
                        <Chevron open={open} />
                      </span>
                    </div>
                    <p className="mt-2 text-[15px] font-semibold text-ink">{item.card.specialty}</p>
                    <p className="mt-0.5 text-[13px] text-muted">{item.card.place}</p>
                  </button>

                  <div
                    className={`grid transition-all duration-300 ease-out ${
                      open ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'
                    }`}
                  >
                    <div className="overflow-hidden">
                      <div className="px-4 pb-4 pt-3">
                        <HistorySection title="Обоснование" text={item.card.reasoning} />
                        <div className="my-3 h-px bg-line" aria-hidden />
                        <HistorySection title="По оплате (ОМС/ДМС/платно)" text={item.card.paymentNote} />
                        <div className="my-3 h-px bg-line" aria-hidden />
                        <HistorySection title="Что делать и что подготовить" text={item.card.preparationTips} />

                        {answers.length > 0 && (
                          <div className="mt-4">
                            <p className="text-[11px] font-medium tracking-wide text-muted uppercase">
                              Ваши ответы при обращении
                            </p>
                            <dl className="mt-2 flex flex-col gap-2.5">
                              {answers.map((pair, index) => (
                                <div key={`${pair.question}-${index}`}>
                                  <dt className="text-[12px] text-muted">{pair.question}</dt>
                                  <dd className="text-[14px] leading-relaxed text-ink">{pair.answer}</dd>
                                </div>
                              ))}
                            </dl>
                          </div>
                        )}

                        <p className="mt-4 text-[12px] leading-relaxed font-medium text-amber-800">
                          ⚠️ При ухудшении состояния немедленно вызывайте скорую помощь (103/112)
                        </p>
                      </div>
                    </div>
                  </div>
                </article>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

function HistorySection({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <h3 className="text-[13px] font-semibold text-ink">{title}</h3>
      <p className="mt-1.5 whitespace-pre-line text-[13px] leading-relaxed text-[#374151]">{text}</p>
    </section>
  )
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={`size-4 shrink-0 text-[#9CA3AF] transition-transform duration-300 ${open ? 'rotate-180' : ''}`}
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  )
}
